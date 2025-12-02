package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"golang.org/x/sys/unix"

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
	"payment-processing-system/internal/config"
	"payment-processing-system/internal/observability"
)

func main() {
	// --- 1. Configuration and Logging ---
	fallbackLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		fallbackLogger.Error("Failed to load config", "ERROR", err)
		os.Exit(1)
	}
	fallbackLogger.Info("Config loaded", "db_url", "loaded connect DB", "app_port", cfg.Server.Port)
	logger := observability.SetupLogger(cfg.App.Env)
	logger.Info("The application is launched", "env", cfg.App.Env)

	//jwtSecret := os.Getenv("JWT_SECRET")
	jwtSecret := cfg.JWT.JWTSecret
	if jwtSecret == "" || jwtSecret == " " {
		logger.Error("JWT_SECRET is not set")
		os.Exit(1)
	}

	// --- 2. Setting up Observability ---
	shutdownTracer, err := observability.InitTracer(cfg.Jaeger.Port, "payment-gateway")
	if err != nil {
		logger.Error("Failed to initialize tracing", "ERROR", err)
		os.Exit(1)
	}
	defer shutdownTracer(context.Background())

	// --- 3. Security Settings --- //TODO: пока не использую OIDC
	// oidcAuthenticator, err := httphandler.NewOIDCAuthenticator(
	// 	context.Background(),
	// 	cfg.OIDC.URL,
	// 	cfg.OIDC.ClientID,
	// )
	// if err != nil {
	// 	logger.Error("Failed to create OIDC authenticator", "url", cfg.OIDC.URL, "client_id", cfg.OIDC.ClientID, "error", err)
	// 	os.Exit(1)
	// }

	opaMiddleware := opa.NewMiddleware(cfg.OPA.URL, logger)

	// Creating Adapters

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Connecting to PostgreSQL
	repo, err := postgres.NewRepository(ctx, cfg.Postgres.DSN)
	if err != nil {
		logger.Error("failed to connect to postgres", "ERROR", err)
		os.Exit(1)
	}
	// Deferred block for closing PostgreSQL repository
	defer repo.Close()

	logger.Info("successfully connected to postgres")

	// Initializing the Redis client
	rateLimiterRepo, err := redis.NewRateLimiterAdapter(cfg.Redis.Addr)
	if err != nil {
		logger.Error("Failed to connect to Redis", "ERROR", err)
		os.Exit(1)
	}

	defer func() {
		if err := rateLimiterRepo.Close(); err != nil {
			logger.Error("error closing Redis connection", "ERROR", err)
		}
	}()

	logger.Info("successfully connected to redis")
	rateLimiterMiddleware := httphandler.NewRateLimiterMiddleware(rateLimiterRepo, logger)

	// Create a Kafka producer
	//TODO: закомментировать для локальной версии тестов
	broker, err := kafka.NewBroker([]string{cfg.Kafka.BootstrapServers}, cfg.Kafka.Topic, logger)

	//TODO: Создаем заглушку для Kafka (для локальной разработки)
	//broker, err := mock.NewBroker([]string{cfg.Kafka.BootstrapServers}, cfg.Kafka.Topic, logger)
	if err != nil {
		logger.Error("Failed to create kafka broker", "ERROR", err)
		os.Exit(1)
	}
	defer broker.Close()
	logger.Info("kafka broker created")

	// Dependency Injection: "Injecting" adapters into the kernel
	transactionService := app.NewTransactionService(repo, broker)
	transactionHandler := httphandler.NewTransactionHandler(transactionService, logger)

	authHandler := httphandler.NewAuthHandler(logger, jwtSecret)

	// Setting up and running an HTTP server
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(rateLimiterMiddleware.Handler)

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(observability.NewLoggerMiddleware(logger))
	r.Use(middleware.Recoverer)
	r.Use(observability.NewMetricsMiddleware("payment-gateway"))
	r.Use(observability.NewTracingMiddleware("payment-gateway"))

	// Public endpoint for login
	r.Post("/login", authHandler.HandleLogin)

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status": "healthy", "service": "payment-gateway"}`)); err != nil {
			logger.Error("Failed to write health response", "ERROR", err)
		}
	})
	// Transaction endpoint
	//r.Post("/transaction", transactionHandler.HandleCreateTransaction)

	r.Handle("/metrics", promhttp.Handler())
	r.Route("/api/v1", func(r chi.Router) {
		// r.Use(oidcAuthenticator.Middleware) //TODO: Пока не используем OIDC
		jwtAuth := httphandler.JWTMiddleware([]byte(jwtSecret))
		r.Use(jwtAuth)
		r.Use(opaMiddleware.Authorize)
		r.Post("/transaction", transactionHandler.HandleCreateTransaction)
	})


	// Graceful Shutdown
	srv := &http.Server{
		Addr:         cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	if srv.Addr == "" {
		srv.Addr = "8080"
	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("starting server", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed to start", "ERROR", err)
			os.Exit(1)
		}
	}()
	wg.Wait()

	// We are waiting for the signal to finish (Ctrl+C)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, unix.SIGINT, unix.SIGTERM)
	<-quit
	logger.Info("shutting down server...")

	// 5 seconds to complete current requests
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", "ERROR", err)
	}

	logger.Info("server exited properly")
}
