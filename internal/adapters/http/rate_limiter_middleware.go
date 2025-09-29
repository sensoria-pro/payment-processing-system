package http

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"payment-processing-system/internal/core/ports"
)

// RateLimiterMiddleware - это middleware для ограничения частоты запросов.
type RateLimiterMiddleware struct {
	repo   ports.RateLimiterRepository
	logger *slog.Logger
}

// NewRateLimiterMiddleware создает новый экземпляр middleware.
func NewRateLimiterMiddleware(repo ports.RateLimiterRepository, logger *slog.Logger) *RateLimiterMiddleware {
	return &RateLimiterMiddleware{
		repo:   repo,
		logger: logger,
	}
}

// Handler является основной функцией middleware.
func (m *RateLimiterMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Используем IP-адрес клиента как ключ для ограничения.
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			m.logger.Error("не удалось получить IP-адрес клиента", "error", err)
			// Если не можем определить IP, пропускаем запрос, чтобы не блокировать легитимных пользователей.
			next.ServeHTTP(w, r)
			return
		}

		// TODO: Вынести лимиты в конфигурацию
		limit := 100               // 100 запросов
		window := 1 * time.Minute // в минуту

		// Проверяем, разрешен ли запрос.
		allowed, err := m.repo.IsAllowed(r.Context(), ip, limit, window)
		if err != nil {
			m.logger.Error("ошибка при проверке rate limit в Redis", "error", err)
			// "Fail-open": Если наш rate limiter (Redis) не работает, мы не должны
			// блокировать весь трафик. Лучше пропустить запрос, чем полностью отказать в обслуживании.
			next.ServeHTTP(w, r)
			return
		}

		if !allowed {
			// Если лимит превышен, возвращаем ошибку 429 Too Many Requests.
			writeJSONError(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		// Если всё в порядке, передаем управление следующему обработчику.
		next.ServeHTTP(w, r)
	})
}