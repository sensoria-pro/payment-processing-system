package ports

import (
	"context"

	"github.com/google/uuid"
	"github.com/sensoria-pro/payment-processing-system/internal/core/domain"
)

// TransactionRepository is an "outgoing port". It defines WHAT we want to do with the repository, but not HOW.
//TODO: Реализация может быть для PostgreSQL, in-memory и т.д.
type TransactionRepository interface {
	Save(ctx context.Context, tx domain.Transaction) error
}

// MessageBroker is another outgoing port for sending messages.
type MessageBroker interface {
	PublishTransactionCreated(ctx context.Context, tx domain.Transaction) error
}

// TransactionService is an "incoming port" that defines how the outside world can interact with our kernel.
type TransactionService interface {
	CreateTransaction(ctx context.Context, amount float64, currency, cardNum string, idemKey uuid.UUID) (*domain.Transaction, error)
}
