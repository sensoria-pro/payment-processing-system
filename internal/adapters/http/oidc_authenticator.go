package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)


// ErrorResponse — стандартная структура ошибки
type ErrorResponse struct {
	Error string `json:"error"`
}

// OIDCAuthenticator хранит верификатор токенов
type OIDCAuthenticator struct {
	Verifier *oidc.IDTokenVerifier
	logger   *slog.Logger
}

// NewOIDCAuthenticator создаёт новый OIDC-аутентификатор
func NewOIDCAuthenticator(ctx context.Context, providerURL, clientID string, logger *slog.Logger) (*OIDCAuthenticator, error) {
	if providerURL == "" || clientID == "" {
		return nil, errors.New("OIDC URL and ClientID cannot be empty")
	}

	provider, err := oidc.NewProvider(ctx, providerURL)
	if err != nil {
		logger.Error("Failed to create OIDC provider", "error", err)
		return nil, errors.New("failed to create OIDC provider")
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})

	if logger == nil {
		return nil, errors.New("logger is required")
	}

	return &OIDCAuthenticator{
		Verifier: verifier,
		logger:   logger,
	}, nil
}

// Middleware — HTTP middleware для проверки токена
func (a *OIDCAuthenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			a.writeJSONError(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			a.writeJSONError(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}
		rawToken := parts[1]

		idToken, err := a.Verifier.Verify(r.Context(), rawToken)
		if err != nil {
			a.logger.Warn("Invalid OIDC token", "error", err)
			a.writeJSONError(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		var claims map[string]interface{}
		if err := idToken.Claims(&claims); err != nil {
			a.logger.Error("Failed to extract OIDC claims", "error", err)
			a.writeJSONError(w, "Failed to extract claims", http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), claimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// writeJSONError — отправляет JSON-ошибку и логирует ошибки сериализации
func (a *OIDCAuthenticator) writeJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		a.logger.Error("Failed to write JSON error response", "error", err)
	}
}