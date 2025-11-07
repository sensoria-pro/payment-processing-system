package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"payment-processing-system/internal/config"
	"payment-processing-system/internal/observability"
)

// Simplified webhook structure from Alertmanager
type AlertWebhook struct {
	Alerts []struct {
		Status string `json:"status"`
		Labels struct {
			Alertname string `json:"alertname"`
			Severity  string `json:"severity"`
		} `json:"labels"`
		Annotations struct {
			Summary string `json:"summary"`
		} `json:"annotations"`
	} `json:"alerts"`
}

func alertHandler(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {
	
	var webhook AlertWebhook
    if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
        logger.Error("Failed to decode webhook", "ERROR", err)
        http.Error(w, "Bad Request", http.StatusBadRequest)
        return
    }

	for _, alert := range webhook.Alerts {
		logger.Info("ALERT",
			"STATUS", alert.Status,
			"ALERTNAME", alert.Labels.Alertname,
			"SEVERITY", alert.Labels.Severity,
			"SUMMARY", alert.Annotations.Summary,
		)
	}
	w.WriteHeader(http.StatusOK)
}

func main() {
	// --- Configuration and Logging ---
	var fallbackLogger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	cfg, err := config.Load("configs/config.yml")
	if err != nil {
		fallbackLogger.Error("Failed to load config", "ERROR", err)
		os.Exit(1)
	}

	logger := observability.SetupLogger(cfg.App.Env)
	logger.Info("The alerter-service is launched", "env", cfg.App.Env)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Wrap alertHandler to pass logger
	r.Post("/alert", func(w http.ResponseWriter, req *http.Request) {
		alertHandler(w, req, logger)
	})
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "OK"}); err != nil {
			logger.Error("Failed to write health response", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	})
	port := cfg.Server.PortAlerter
	if port == "" {
		port = "8081"
	}
	logger.Info("Alerter service started on", "PORT", port)

	if err := http.ListenAndServe("0.0.0.0:"+port, r); err != nil {
		logger.Error("Failed to start server", "ERROR", err)
		os.Exit(1)
	}
}