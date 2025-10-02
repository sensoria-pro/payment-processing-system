package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"payment-processing-system/internal/core/ports"
)

// TransactionHandler now stores all its dependencies.
type TransactionHandler struct {
	service ports.TransactionService
	logger  *slog.Logger
}

// NewTransactionHandler now accepts a logger as a dependency.
func NewTransactionHandler(service ports.TransactionService, logger *slog.Logger) *TransactionHandler {
	return &TransactionHandler{
		service: service,
		logger:  logger,
	}
}

type createTransactionRequest struct {
	IdempotencyKey string  `json:"idempotency_key"`
	CardNumber     string  `json:"card_number"`
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
}

func (h *TransactionHandler) HandleCreateTransaction(w http.ResponseWriter, r *http.Request) {
	var req createTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	idemKey, err := uuid.Parse(req.IdempotencyKey)
	if err != nil {
		writeJSONError(w, "invalid idempotency key", http.StatusBadRequest)
		return
	}

	tx, err := h.service.CreateTransaction(r.Context(), req.Amount, req.Currency, req.CardNumber, idemKey)
	if err != nil {
		// TODO: Реализовать более сложную логику для разных типов ошибок от сервиса.
		writeJSONError(w, "failed to create transaction", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202 Accepted
	
	if err := json.NewEncoder(w).Encode(map[string]string{"transaction_id": tx.ID.String()}); err != nil {
		// use the logger that came through the structure.
		h.logger.Error("failed to write json response", "ERROR", err)
	}
}