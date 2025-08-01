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
	"github.com/redis/go-redis/v9"

	httphandler "github.com/sensoria-pro/payment-processing-system/internal/adapters/http"
	"github.com/sensoria-pro/payment-processing-system/internal/adapters/messaging/kafka"
	"github.com/sensoria-pro/payment-processing-system/internal/adapters/messaging/mock"
	"github.com/sensoria-pro/payment-processing-system/internal/adapters/storage/postgres"
	"github.com/sensoria-pro/payment-processing-system/internal/app"
	"github.com/sensoria-pro/payment-processing-system/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Creating Adapters
	// Connecting to PostgreSQL
	repo, err := postgres.NewRepository(ctx, cfg.Postgres.DSN)
	if err != nil {
		logger.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer repo.Close()
	logger.Info("successfully connected to postgres")

	// Create a Kafka producer
	//TODO: закомментировать для локальной версии тестов
	//broker, err := kafka.NewBroker(cfg.Kafka.BootstrapServers, cfg.Kafka.Topic)

	//TODO: Создаем заглушку для Kafka (для локальной разработки)
	broker, err := mock.NewBroker(cfg.Kafka.BootstrapServers, cfg.Kafka.Topic)
	if err != nil {
		logger.Error("failed to create kafka broker", "error", err)
		os.Exit(1)
	}
	defer broker.Close()
	logger.Info("kafka broker created")

	// Dependency Injection: "Injecting" adapters into the kernel
	transactionService := app.NewTransactionService(repo, broker)
	transactionHandler := httphandler.NewTransactionHandler(transactionService)

	// Initializing the Redis client
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr})
	defer rdb.Close()

	// Setting up and running an HTTP server
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy", "service": "payment-gateway"}`))
	})
	// Transaction endpoint
	r.Post("/transaction", transactionHandler.HandleCreateTransaction)

	srv := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: r,
	}

	// Graceful Shutdown
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("starting server", "port", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed to start", "error", err)
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
		logger.Error("server shutdown failed", "error", err)
	}

	logger.Info("server exited properly")
}
