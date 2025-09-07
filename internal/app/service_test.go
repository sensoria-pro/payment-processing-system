package app

import (
	"context"

	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"payment-processing-system/internal/core/domain"
	_ "payment-processing-system/internal/core/ports"
)

// Mock - implementation of the repository
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) Save(ctx context.Context, tx domain.Transaction) error {
	args := m.Called(ctx, tx)
	return args.Error(0)
}

// Mock - implementation of a broker
type MockBroker struct {
	mock.Mock
}

func (m *MockBroker) PublishTransactionCreated(ctx context.Context, tx domain.Transaction) error {
	args := m.Called(ctx, tx)
	return args.Error(0)
}
//first test
func TestTransactionService_CreateTransaction_Success(t *testing.T) {
	// --- Arrange ---
	mockRepo := new(MockRepository)
	mockBroker := new(MockBroker)

	// We create a service by implementing our mocks into it
	//TODO: Мы еще не создали 'NewTransactionService', так что это RED-фаза
	service := NewTransactionService(mockRepo, mockBroker)

	ctx := context.Background()
	idemKey := uuid.New()
	cardNum := "4000123456789010"

	// We expect the Save method to be called 1 time with any transaction object
	mockRepo.On("Save", ctx, mock.AnythingOfType("domain.Transaction")).Return(nil)
	// And we expect that the Publish method will be called 1 time
	mockBroker.On("PublishTransactionCreated", ctx, mock.AnythingOfType("domain.Transaction")).Return(nil)

	// --- Act ---
	result, err := service.CreateTransaction(ctx, 100.0, "RUB", cardNum, idemKey)

	// --- Assert ---
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, domain.StatusProcessing, result.Status)
	assert.NotEmpty(t, result.ID)
	assert.NotEmpty(t, result.CardNumberHash)
	assert.NotEqual(t, cardNum, result.CardNumberHash) //TODO: Убедимся, что номер карты захэширован

	// Check that the mocks were called as we expected
	mockRepo.AssertExpectations(t)
	mockBroker.AssertExpectations(t)
}

// second test
func TestTransactionService_CreateTransaction_InvalidAmount(t *testing.T) {
	// --- Arrange ---
	mockRepo := new(MockRepository)
	mockBroker := new(MockBroker)
	service := NewTransactionService(mockRepo, mockBroker)
	ctx := context.Background()

	// --- Act ---
	_, err := service.CreateTransaction(ctx, -50.0, "RUB", "1234", uuid.New())

	// --- Assert ---
	assert.Error(t, err) // We are expecting an error
	// Let's make sure that neither the repository nor the broker were called
	mockRepo.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	mockBroker.AssertNotCalled(t, "PublishTransactionCreated", mock.Anything, mock.Anything)
}
