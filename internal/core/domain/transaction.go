package domain

import (
	"time"
	"github.com/google/uuid"
)

// TransactionStatus is our own type for statuses to avoid "magic strings".
type TransactionStatus string

const (
	StatusProcessing TransactionStatus = "PROCESSING"
	StatusCompleted  TransactionStatus = "COMPLETED"
	StatusFailed     TransactionStatus = "FAILED"
)

// Transaction is the central entity of our domain.
//TODO: Она не содержит тегов для JSON или БД, это чистая бизнес-модель.
type Transaction struct {
	ID              uuid.UUID
	Status          TransactionStatus
	Amount          float64
	Currency        string
	CardNumberHash  string //TODO: Хэш номера карты, а не сам номер
	IdempotencyKey  uuid.UUID
	CreatedAt       time.Time
}