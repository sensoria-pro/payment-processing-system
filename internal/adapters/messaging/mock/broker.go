package mock

import (
	"context"
	"fmt"
	"github.com/sensoria-pro/payment-processing-system/internal/core/domain"
)

//Broker - stub for MessageBroker
type Broker struct{}

func NewBroker(bootstrapServers, topic string) (*Broker, error) {
	return &Broker{}, nil
}

func (b *Broker) Close() error {
	return nil
}

func (b *Broker) PublishTransactionCreated(ctx context.Context, tx domain.Transaction) error {
	//TODO: Пока просто логируем сообщение вместо отправки в Kafka
	fmt.Printf("📨 [MOCK] Transaction created: %s, Amount: %.2f %s\n",
		tx.ID.String(), tx.Amount, tx.Currency)
	return nil
}
