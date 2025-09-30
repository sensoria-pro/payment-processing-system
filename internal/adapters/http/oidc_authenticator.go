package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

// Key for storing claims in context
type contextKey string
var logger *slog.Logger
const claimsContextKey = contextKey("claims")

// ErrorResponse is a standard structure for returning errors in JSON format.
type ErrorResponse struct {
	Error string `json:"error"`
}



// OIDCAuthenticator stores the token verifier.
type OIDCAuthenticator struct {
	Verifier *oidc.IDTokenVerifier
	logger *slog.Logger
}

// NewOIDCAuthenticator connects to the OIDC provider (Keycloak) and creates an authenticator.
func NewOIDCAuthenticator(ctx context.Context, providerURL, clientID string, logger *slog.Logger) (*OIDCAuthenticator, error) {
	if providerURL == "" || clientID == "" {
		return nil, fmt.Errorf("OIDC URL and ClientID cannot be empty")
	}

	provider, err := oidc.NewProvider(ctx, providerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})

	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	return &OIDCAuthenticator{
		Verifier: verifier,
		logger:   logger,
	}, nil
}

// writeJSONError is a helper for sending errors in JSON format.
func writeJSONError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	
	if err := json.NewEncoder(w).Encode(ErrorResponse{Error: message}); err != nil {
		logger.Error("faled to encode JSON response: %v", err)
	}
}

// Middleware - This is an HTTP middleware for token verification.
func (a *OIDCAuthenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSONError(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeJSONError(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}
		rawToken := parts[1]

		// Verifying the token
		idToken, err := a.Verifier.Verify(r.Context(), rawToken)
		if err != nil {
			writeJSONError(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
			return
		}

		// Extracting claims (data) from the token
		var claims map[string]interface{}
		if err := idToken.Claims(&claims); err != nil {
			writeJSONError(w, "Failed to extract claims: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Saving claims in context for OPA
		ctx := context.WithValue(r.Context(), claimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
