package domain

import (
	"time"
	"github.com/google/uuid"
)
// Package domain contains the core business logic and models for the application.
// This package has NO dependencies on external libraries like databases, Kafka, Redis, etc.
// It is the pure, technology-agnostic heart of the service.

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

// FraudResult represents the outcome of a fraud check.
type FraudResult struct {
	IsFraudulent bool   `json:"is_fraudulent"`
	Reason       string `json:"reason,omitempty"`
}

// FraudRuleEngine is an interface (a "port" in Hexagonal Architecture).
// It defines the contract for any component that can check a transaction for fraud.
// The core logic doesn't care HOW the check is performed (in-memory, Redis, external service),
// only that it can be done.
type FraudRuleEngine interface {
	CheckTransaction(tx Transaction) FraudResult
}