package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"
	"errors"

	"github.com/google/uuid"
	"payment-processing-system/internal/core/domain"
	"payment-processing-system/internal/core/ports"
)

// service is the implementation of the TransactionService port
type service struct {
	repo   ports.TransactionRepository
	broker ports.MessageBroker
}

// NewTransactionService is the constructor of our service.
//TODO: Он принимает зависимости через интерфейсы (Dependency Injection).
func NewTransactionService(repo ports.TransactionRepository, broker ports.MessageBroker) ports.TransactionService {
	return &service{
		repo:   repo,
		broker: broker,
	}
}

func (s *service) CreateTransaction(ctx context.Context, amount float64, currency, cardNum string, idemKey uuid.UUID) (*domain.Transaction, error) {
	// TODO: Добавить валидацию (например, amount > 0)

	// Hashing the card number
	hash := sha256.Sum256([]byte(cardNum))
	cardHash := fmt.Sprintf("%x", hash)

	tx := domain.Transaction{
		ID:             uuid.New(),
		Status:         domain.StatusProcessing,
		Amount:         amount,
		Currency:       currency,
		CardNumberHash: cardHash,
		IdempotencyKey: idemKey,
		CreatedAt:      time.Now(),
	}

	//TODO: Ранняя валидация (Refactor)
	if amount <= 0 {
		return nil, errors.New("amount must be positive")
	}

	if err := s.repo.Save(ctx, tx); err != nil {
		// TODO: Добавить более гранулярную обработку ошибок
		return nil, err
	}

	if err := s.broker.PublishTransactionCreated(ctx, tx); err != nil {
		// TODO: Что делать, если транзакция сохранилась, а сообщение не отправилось, попробовать Паттерн Outbox
		return nil, err
	}

	return &tx, nil
}
