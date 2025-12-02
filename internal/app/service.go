package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"payment-processing-system/internal/core/domain"
	"payment-processing-system/internal/core/ports"

	"github.com/google/uuid"
)

// service is the implementation of the TransactionService port
type service struct {
	repo   ports.TransactionRepository
	broker ports.MessageBroker
}

// NewTransactionService is the constructor of our service.
// TODO: Он принимает зависимости через интерфейсы (Dependency Injection).
func NewTransactionService(repo ports.TransactionRepository, broker ports.MessageBroker) ports.TransactionService {
	return &service{
		repo:   repo,
		broker: broker,
	}
}

// isValidCard validates a card number using the Luhn algorithm
func isValidCard(cardNum string) bool {
	// Remove spaces and check if all characters are digits
	cardNum = strings.ReplaceAll(cardNum, " ", "")
	if len(cardNum) < 13 || len(cardNum) > 19 {
		return false
	}

	// Check if all characters are digits
	sum := 0
	isSecond := false

	// Process digits from right to left
	for i := len(cardNum) - 1; i >= 0; i-- {
		digit, err := strconv.Atoi(string(cardNum[i]))
		if err != nil {
			return false // Non-digit character found
		}

		if isSecond {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}

		sum += digit
		isSecond = !isSecond
	}

	// Card is valid if sum is divisible by 10
	return sum%10 == 0
}

func (s *service) CreateTransaction(ctx context.Context, amount float64, currency, cardNum string, idemKey uuid.UUID) (*domain.Transaction, error) {
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

	if amount <= 0 {
		return nil, domain.ErrInvalidAmount
	}

	if !isValidCard(cardNum) {
		return nil, domain.ErrInvalidCard
	}

	if err := s.repo.Save(ctx, tx); err != nil {
		if errors.Is(err, domain.ErrIdempotencyKeyUsed) {
			return nil, err
		}
		return nil, domain.ErrStorageUnavailable
	}

	if err := s.broker.PublishTransactionCreated(ctx, tx); err != nil {
		return nil, domain.ErrBrokerUnavailable
	}

	return &tx, nil
}
