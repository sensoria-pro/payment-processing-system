package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"payment-processing-system/internal/core/domain"
)

// Broker is an implementation of the MessageBroker port for Kafka.
type Broker struct {
	client *kgo.Client
	topic  string
	logger *slog.Logger
	wg     sync.WaitGroup
}

// NewBroker creates a new Kafka broker instance.
func NewBroker(bootstrapServers []string, topic string, logger *slog.Logger) (*Broker, error) {
	opts := []kgo.Opt{
		kgo.SeedBrokers(bootstrapServers...),
		kgo.DefaultProduceTopic(topic),
		kgo.AllowAutoTopicCreation(), //TODO: Удобно для локальной разработки
		kgo.RequiredAcks(kgo.AllISRAcks()),  //TODO: Гарантируем, что сообщение получено всеми репликами
		kgo.RecordDeliveryTimeout(10 * time.Second),
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать kafka-клиент: %w", err)
	}

	// Checking the connection
	if err := client.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("не удалось подключиться к kafka: %w", err)
	}

	return &Broker{
		client: client,
		topic:  topic,
		logger: logger,
	}, nil
}

// PublishTransactionCreated publishes an event about the creation of a transaction.
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

	record := &kgo.Record{
		Key:   []byte(tx.ID.String()),
		Value: payload,
	}

	b.wg.Add(1)
	// Produce sends a record asynchronously.
	b.client.Produce(ctx, record, func(r *kgo.Record, err error) {
		defer b.wg.Done()
		if err != nil {
			b.logger.Error("не удалось доставить сообщение в kafka", "topic", r.Topic, "error", err)
		} else {
			b.logger.Debug("сообщение успешно доставлено в kafka", "topic", r.Topic, "partition", r.Partition, "offset", r.Offset)
		}
	})

	return nil
}
// Close gracefully stops the producer.
func (b *Broker) Close() {
	b.logger.Info("ожидание завершения отправки сообщений в kafka...")
	b.wg.Wait() // Ждём, пока все колбэки отработают
	b.client.Close()
	b.logger.Info("kafka-клиент успешно остановлен")
}
