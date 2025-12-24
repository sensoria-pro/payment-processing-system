package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// JWTMiddleware проверяет JWT и сохраняет claims в контекст
func JWTMiddleware(jwtSecret []byte, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSONError(w, "Authorization header required", http.StatusUnauthorized, logger)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				writeJSONError(w, "Invalid Authorization header format", http.StatusUnauthorized, logger)
				return
			}

			tokenString := parts[1]

			// Безопасная верификация токена
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				// Явно разрешаем только HS256
				if token.Method.Alg() != "HS256" {
					return nil, errors.New("unexpected signing method")
				}
				return jwtSecret, nil
			})

			if err != nil {
				logger.Warn("JWT validation failed", "error", err)
				writeJSONError(w, "Invalid token", http.StatusUnauthorized, logger)
				return
			}
			if !token.Valid {
				logger.Warn("JWT token is not valid")
				writeJSONError(w, "Invalid token", http.StatusUnauthorized, logger)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				logger.Warn("Failed to cast token claims")
				writeJSONError(w, "Invalid token claims", http.StatusUnauthorized, logger)
				return
			}

			// Сохраняем claims в типизированный контекст
			ctx := context.WithValue(r.Context(), claimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// writeJSONError — вспомогательная функция для отправки JSON-ошибок
func writeJSONError(w http.ResponseWriter, message string, status int, logger *slog.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		logger.Error("Failed to write JSON error response", "error", err)
	}
}