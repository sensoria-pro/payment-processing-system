package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"payment-processing-system/internal/adapters/auth/opa"
	httphandler "payment-processing-system/internal/adapters/http"
	"payment-processing-system/internal/adapters/messaging/kafka"
	_ "payment-processing-system/internal/adapters/messaging/mock"
	"payment-processing-system/internal/adapters/storage/postgres"
	"payment-processing-system/internal/adapters/storage/redis"
	"payment-processing-system/internal/app"
	"payment-processing-system/internal/auth"
	"payment-processing-system/internal/config"
	"payment-processing-system/internal/observability"
)

func main() {
	// --- 1. Configuration and Logging ---
	fallbackLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		fallbackLogger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	logger := observability.SetupLogger(cfg.App.Env)
	logger.Info("Application starting", "env", cfg.App.Env, "port", cfg.Server.Port)

	// --- 2. Validate critical config ---
	jwtSecret := cfg.JWT.JWTSecret
	if jwtSecret == "" {
		logger.Error("JWT_SECRET is not set")
		os.Exit(1)
	}

	// --- 3. Observability ---
	shutdownTracer, err := observability.InitTracer(cfg.Jaeger.Port, "payment-gateway")
	if err != nil {
		logger.Error("Failed to initialize tracing", "ERROR", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdownTracer(context.Background()); err != nil {
			logger.Warn("Failed to shutdown tracer", "ERROR", err)
		}
	}()

	// --- 4. Dependencies ---
	ctx := context.Background()

	repo, err := postgres.NewRepository(ctx, cfg.Postgres.DSN)
	if err != nil {
		logger.Error("Failed to connect to PostgreSQL", "ERROR", err)
		os.Exit(1)
	}
	defer repo.Close()
	logger.Info("Connected to PostgreSQL")

	// Redis
	rateLimiterRepo, err := redis.NewRateLimiterAdapter(cfg.Redis.Addr)
	if err != nil {
		logger.Error("Failed to connect to Redis", "ERROR", err)
		os.Exit(1)
	}
	defer func() {
		if err := rateLimiterRepo.Close(); err != nil {
			logger.Info("connected to Redis", "ERROR", err)
		}
	}()

	// Kafka
	broker, err := kafka.NewBroker([]string{cfg.Kafka.BootstrapServers}, cfg.Kafka.Topic, logger)
	if err != nil {
		logger.Error("Failed to create Kafka broker", "ERROR", err)
		os.Exit(1)
	}
	defer broker.Close()
	logger.Info("Kafka broker created")

	// --- 5. Service Layer ---
	transactionService := app.NewTransactionService(repo, broker)
	transactionHandler := httphandler.NewTransactionHandler(transactionService, logger)
	// authHandler := httphandler.NewAuthHandler(logger, jwtSecret)
	rateLimiterMiddleware := httphandler.NewRateLimiterMiddleware(rateLimiterRepo, logger)
	opaMiddleware := opa.NewMiddleware(cfg.OPA.URL, logger)
	oauthServer := auth.NewAuthorizationServer(jwtSecret, logger)

	// --- 6. HTTP Router ---
	r := chi.NewRouter()

	// Public middleware
	r.Use(
		middleware.RequestID,
		middleware.RealIP,
		rateLimiterMiddleware.Handler,
		middleware.Logger,
		middleware.Recoverer,
		observability.NewLoggerMiddleware(logger),
		observability.NewMetricsMiddleware("payment-gateway"),
		observability.NewTracingMiddleware("payment-gateway"),
	)

	// Public routes
	// r.Post("/login", authHandler.HandleLogin)
	r.Post("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		err := oauthServer.HandleTokenRequest(w, r)
		if err != nil {
			logger.Error("failed to handle token request", "error", err)
		}
	})
	// Health check
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status":  "healthy",
			"service": "payment-gateway",
		}); err != nil {
			logger.Error("Failed to write health response", "error", err)
		}
	})
	r.Handle("/metrics", promhttp.Handler())

	// Protected routes: /api/v1/*
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(
			httphandler.JWTMiddleware([]byte(jwtSecret), logger),
			opaMiddleware.Authorize,
		)
		r.Post("/transaction", transactionHandler.HandleCreateTransaction)
	})

	// Protected routes: /profile (example)
	r.Group(func(r chi.Router) {
		r.Use(httphandler.JWTMiddleware([]byte(jwtSecret), logger))
		r.Get("/profile", func(w http.ResponseWriter, r *http.Request) {
			userIDRaw := r.Context().Value("userID")
			if userID, ok := userIDRaw.(string); ok && userID != "" {
				_, _ = w.Write([]byte("Your user ID: " + userID))
				return
			}
			http.Error(w, "Failed to get user ID", http.StatusUnauthorized)
		})
	})

	// --- 7. HTTP Server ---
	serverAddr := cfg.Server.Port
	if serverAddr == "" {
		serverAddr = "8080"
	}

	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server
	go func() {
		logger.Info("HTTP server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Server exited properly")
}