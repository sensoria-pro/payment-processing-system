package http

// contextKey — типизированный ключ для контекста
type contextKey string

// claimsContextKey — ключ для хранения claims (JWT или OIDC) в контексте
const claimsContextKey contextKey = "claims"