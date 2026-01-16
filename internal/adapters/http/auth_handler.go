package http

import (
	"encoding/json"
	"net/http"
	"time"
	"log/slog"

	"github.com/golang-jwt/jwt/v5"
)

// AuthHandler handles authentication-related requests.
type AuthHandler struct {
	jwtSecret []byte
	logger   *slog.Logger
}

// NewAuthHandler creates a new AuthHandler instance.
func NewAuthHandler(logger *slog.Logger, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		logger:    logger,
		jwtSecret: []byte(jwtSecret),	
	}
}

// LoginRequest - structure for login request.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"` //TODO: реализовать проверку пароля
}

// LoginResponse - structure for response with token.
type LoginResponse struct {
	Token string `json:"token"`
}

// HandleLogin is our main method for login.
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	//TODO: временная имитация - Реализовать запрос в базу данных.
	var roles []string
	var userID string
	switch req.Username {
	case "admin":
		roles = []string{"admin", "customer"}
		userID = "user-admin-123"
	case "customer":
		roles = []string{"customer"}
		userID = "user-customer-456"
	default:
		h.writeJSONError(w, "Invalid username", http.StatusUnauthorized)
		return
	}
	// Create a JWT token
	claims := jwt.MapClaims{
		"sub":   userID,                               // Subject (user ID)
		"roles": roles,                                // Custom roles for OPA
		"exp":   time.Now().Add(time.Hour * 1).Unix(), // Token lifespan is 1 hour
		"iat":   time.Now().Unix(),                    // Token creation time
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with our secret
	tokenString, err := token.SignedString(h.jwtSecret)
	if err != nil {
		h.writeJSONError(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Send the token to the client
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(LoginResponse{Token: tokenString}); err != nil {
		// If we can't send a response, we log it
		h.logger.Error("failed to write json response", "ERROR", err)
	}
}

func (h *AuthHandler) writeJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		h.logger.Error("Failed to write JSON error response", "error", err)
	}
}
