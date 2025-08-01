package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/sensoria-pro/payment-processing-system/internal/core/domain"
	//TODO: Нужен логгер, пока просто воспользуемся стандартным
	"log" 
)

// Broker is an implementation of the MessageBroker port for Kafka.
type Broker struct {
	producer *kafka.Producer
	topic    string
}

// NewBroker creates a new Kafka broker instance.
func NewBroker(bootstrapServers, topic string) (*Broker, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": bootstrapServers,
		"client.id":         "payment-gateway",
		"acks":              "all", //TODO: Гарантируем, что сообщение получено всеми репликами
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka producer: %w", err)
	}

	b := &Broker{
		producer: p,
		topic:    topic,
	}

	// Start a goroutine to handle delivery reports.
	//!!! This is critical for asynchronous operation.
	go b.handleDeliveryReports()

	return b, nil
}

func (b *Broker) handleDeliveryReports() {
	for e := range b.producer.Events() {
		switch ev := e.(type) {
		case *kafka.Message:
			if ev.TopicPartition.Error != nil {
				log.Printf("❌ Failed to deliver message: %v\n", ev.TopicPartition)
			} else {
				log.Printf("✅ Delivered message to %v\n", ev.TopicPartition)
			}
		}
	}
}

// PublishTransactionCreated implements the MessageBroker interface method.
func (b *Broker) PublishTransactionCreated(ctx context.Context, tx domain.Transaction) error {
	// Creating a message structure for Kafka
	message := map[string]interface{}{
		"transaction_id":   tx.ID.String(),
		"amount":           tx.Amount,
		"currency":         tx.Currency,
		"card_number_hash": tx.CardNumberHash,
		"status":           string(tx.Status),
		"idempotency_key":  tx.IdempotencyKey.String(),
		"created_at":       tx.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %w", err)
	}

	// producer.Produce is an asynchronous call. It does not block execution.
	err = b.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &b.topic, Partition: kafka.PartitionAny},
		Key:            []byte(tx.ID.String()), // Use transaction ID as key
		Value:          payload,
	}, nil) //TODO: nil - deliveryChan, мы обрабатываем глобально в goroutine

	if err != nil {
		return fmt.Errorf("failed to produce message to kafka: %w", err)
	}

	return nil
}

// Close gracefully shutsdown the producer.
func (b *Broker) Close() {
	//TODO: Flush ждет, пока все сообщения в очереди будут отправлены.
	b.producer.Flush(15 * 1000)
	b.producer.Close()
}
