package http

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/sensoria-pro/payment-processing-system/internal/core/ports"
)

type TransactionHandler struct {
	service ports.TransactionService
}

func NewTransactionHandler(service ports.TransactionService) *TransactionHandler {
	return &TransactionHandler{service: service}
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
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	idemKey, err := uuid.Parse(req.IdempotencyKey)
	if err != nil {
		http.Error(w, "invalid idempotency key", http.StatusBadRequest)
		return
	}

	tx, err := h.service.CreateTransaction(r.Context(), req.Amount, req.Currency, req.CardNumber, idemKey)
	if err != nil {
		//TODO: Реализовать более сложную логику для разных типов ошибок от сервиса.
		http.Error(w, "failed to create transaction", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202 Accepted
	json.NewEncoder(w).Encode(map[string]string{"transaction_id": tx.ID.String()})
}