package auth

import (
	"log/slog"

	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/errors"
	"github.com/go-oauth2/oauth2/v4/generates"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/models"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/go-oauth2/oauth2/v4/store"
	"github.com/golang-jwt/jwt/v5"
)

// NewAuthorizationServer creates and configures a new OAuth 2.0 server.
func NewAuthorizationServer(jwtSecret string, logger *slog.Logger) *server.Server {
	manager := manage.NewDefaultManager()

	// token store
	manager.MustTokenStorage(store.NewMemoryTokenStore())

	// Configure the token generator to use JWT.
	manager.MapAccessGenerate(generates.NewJWTAccessGenerate("", []byte(jwtSecret), jwt.SigningMethodHS256))

	// Create an in-memory client store.
	clientStore := store.NewClientStore()
	err := clientStore.Set("test-client", &models.Client{
		ID:     "test-client",
		Secret: "test-secret",
		Domain: "http://localhost",
	})
	if err != nil {
		logger.Error("failed to set client in store", "error", err)
		return nil
	}
	manager.MapClientStorage(clientStore)

	// Create the OAuth 2.0 server.
	srv := server.NewServer(server.NewConfig(), manager)

	// We are using the Client Credentials grant type.
	srv.SetAllowGetAccessRequest(true)
	srv.SetClientInfoHandler(server.ClientFormHandler)

	// Add custom claims to the token.
	srv.SetExtensionFieldsHandler(func(ti oauth2.TokenInfo) (fieldsValue map[string]interface{}) {
		fieldsValue = map[string]interface{}{
			"sub":   ti.GetClientID(), // Use client_id as the subject for M2M
			"roles": []string{"service"}, // Assign a default role for services
		}
		return
	})

	// Internal error handler.
	srv.SetInternalErrorHandler(func(err error) (re *errors.Response) {
		logger.Error("Internal OAuth2 server error", "error", err)
		return
	})

	logger.Info("OAuth 2.0 server configured successfully")
	return srv
}
