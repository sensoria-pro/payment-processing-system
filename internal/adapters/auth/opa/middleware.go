package opa

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Key for extracting claims from context
type contextKey string

const claimsContextKey = contextKey("claims")

// Middleware for authorization via OPA.
type Middleware struct {
	opaURL string
	logger *slog.Logger
	client *http.Client
}

// NewMiddleware creates a new OPA middleware.
func NewMiddleware(opaURL string, logger *slog.Logger) *Middleware {
	return &Middleware{
		opaURL: opaURL,
		logger: logger,
		client: &http.Client{Timeout: 500 * time.Millisecond},
	}
}

// OPAInput - structure for querying OPA.
type OPAInput struct {
	Method string                 `json:"method"`
	Path   string                 `json:"path"`
	User   map[string]interface{} `json:"user"`
}

// OPAResponse - structure for response from OPA.
type OPAResponse struct {
	Allow bool `json:"allow"`
}

// Authorize is an HTTP middleware that performs permissions checking.
func (m *Middleware) Authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(claimsContextKey).(map[string]interface{})
		if !ok {
			http.Error(w, "Claims not found in context", http.StatusInternalServerError)
			return
		}

		// Generate input for OPA
		input := OPAInput{
			Method: r.Method,
			Path:   r.URL.Path,
			User:   claims,
		}

		inputBytes, err := json.Marshal(map[string]interface{}{"input": input})
		if err != nil {
			m.logger.Error("Failed to create OPA request", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Make a request to OPA
		// The URL typically looks like http://opa:8181/v1/data/httpapi/authz
		req, err := http.NewRequestWithContext(r.Context(), "POST", m.opaURL, bytes.NewBuffer(inputBytes))
		if err != nil {
			m.logger.Error("Failed to create OPA request", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := m.client.Do(req)
		if err != nil {
			m.logger.Error("error accessing OPA", "error", err)
			http.Error(w, "Authorization service unavailable", http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()

		var opaResp OPAResponse
		if err := json.NewDecoder(resp.Body).Decode(&opaResp); err != nil {
			m.logger.Error("Unable to decrypt response from OPA", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Checking the OPA solution
		if !opaResp.Allow {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
