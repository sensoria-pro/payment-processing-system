package http

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"time"

	"payment-processing-system/internal/core/ports"
)

// RateLimiterMiddleware - This is a middleware for limiting the request rate.
type RateLimiterMiddleware struct {
	repo   ports.RateLimiterRepository
	logger *slog.Logger
}
// NewRateLimiterMiddleware creates a new middleware instance.
func NewRateLimiterMiddleware(repo ports.RateLimiterRepository, logger *slog.Logger) *RateLimiterMiddleware {
	return &RateLimiterMiddleware{
		repo:   repo,
		logger: logger,
	}
}

// Handler is the main function of the middleware.
func (m *RateLimiterMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use the client's IP address as the key for restriction.
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			m.logger.Error("failed to obtain client IP address", "ERROR", err)
			// If we can't determine the IP, we skip the request so as not to block legitimate users.
			next.ServeHTTP(w, r)
			return
		}

		// TODO: Вынести лимиты в конфигурацию
		limit := 100               // 100 запросов
		window := 1 * time.Minute // в минуту

		// Check if the request is allowed.
		allowed, err := m.repo.IsAllowed(r.Context(), ip, limit, window)
		if err != nil {
			m.logger.Error("ошибка при проверке rate limit в Redis", "ERROR", err)
			// "Fail-open": If our rate limiter (Redis) is not working, we should not
			// Block all traffic. It's better to allow a request than to deny service completely.
			next.ServeHTTP(w, r)
			return
		}

		if !allowed {
			// If the limit is exceeded, return the error 429 Too Many Requests.
			m.writeJSONError(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		// If everything is in order, pass control to the next handler.
		next.ServeHTTP(w, r)
	})
}

func (m *RateLimiterMiddleware) writeJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		m.logger.Error("Failed to write JSON error response", "error", err)
	}
}