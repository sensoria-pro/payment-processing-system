package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"

	"payment-processing-system/internal/antifraud"
	"payment-processing-system/internal/config"
	"payment-processing-system/internal/core/domain"
	"payment-processing-system/internal/observability"
)

// dlqTopic is the name of our Dead-Letter Queue topic.
var dlqTopic = "transactions.created.dlq"

func main() {
	// --- Configuration Setup ---
	cfg, err := config.Load("configs/config.yml")
	
	logger := observability.SetupLogger(cfg.App.Env)
	logger.Info("anti-fraud analyzer запускается", "env", cfg.App.Env)

	if err != nil {
		logger.Error("Failed to load config", "ERROR", err)
		os.Exit(1)
	}

	// Load configuration from environment variables with sensible defaults for local development.

	// --- Component Initialization ---
	kafkaBrokers := strings.Split(cfg.Kafka.BootstrapServers, ",")

	// Kafka Producer (for sending to DLQ)
	dlqProducer, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaBrokers...),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		logger.Error("failed to create Kafka producer for DLQ", "error", err)
		os.Exit(1)
	}
	defer dlqProducer.Close()

	// ClickHouse Client: For writing fraud analysis results.
	chConn, err := clickhouse.Open(&clickhouse.Options{Addr: []string{cfg.ClickHouse.Addr}})
	if err != nil {
		logger.Error("failed to connect to ClickHouse", "error", err)
		os.Exit(1)
	}

	defer func() {
		if err := chConn.Close(); err != nil {
			logger.Error("Failed to close ClickHouse connection", "error", err)
		}
	}()

	// Redis Client: Dependency for our caching rule engine.
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr})

	defer func() {
		if err := rdb.Close(); err != nil {
			logger.Error("Failed to close redis connection", "error", err)
		}
	}()

	// Fraud Rule Engine: Instantiate our chosen rule engine implementation.
	// Thanks to the interface, we could easily swap this for a different engine.
	ruleEngine := antifraud.NewCachingRuleEngine(rdb, cfg.AntiFraud)

	// --- Application Start ---

	// Subscribe to the main transaction topic.
	consumerClient, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaBrokers...),
		kgo.ConsumerGroup("anti-fraud-group"),
		kgo.ConsumeTopics("transactions.created"),
		kgo.DisableAutoCommit(), //TODO: Мы будем коммитить offset'ы вручную для большей надежности
	)
	if err != nil {
		logger.Error("failed to create Kafka consumer:", "error", err)
		os.Exit(1)
	}
	defer consumerClient.Close()

	// Set up graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("anti-fraud analyzer запущен и готов к работе...")

	// Main processing loop.
	run := true
	for run {
		select {
		case <-ctx.Done(): // Exit loop on shutdown signal.
			run = false
		default:
			fetches := consumerClient.PollFetches(ctx)
			// Проверяем, не был ли клиент закрыт или контекст отменен
			if fetches.IsClientClosed() || ctx.Err() != nil {
				break // Выходим из цикла для грациозной остановки
			}
			
			fetches.EachError(func(t string, p int32, err error) {
				logger.Error("ошибка при чтении из kafka", "topic", t, "partition", p, "error", err)
			})
			fetches.EachRecord(func(record *kgo.Record) {
				var tx domain.Transaction
				if err := json.Unmarshal(record.Value, &tx); err != nil {
					logger.Error("Не удалось распарсить сообщение. Отправка в DLQ.", "ERROR", err)
					sendToDLQ(dlqProducer, record, "unmarshal_error", err.Error())
					return // Пропускаем обработку этого сообщения
				}

				// Apply our fraud rules to the transaction.
				result := ruleEngine.CheckTransaction(tx)

				// Persist the analysis result to ClickHouse.
				err = chConn.Exec(ctx, `
				INSERT INTO default.fraud_reports (transaction_id, is_fraudulent, reason, card_hash, amount, processed_at) VALUES (?, ?, ?, ?, ?, ?)`,
				tx.ID,
				result.IsFraudulent,
				result.Reason,
				tx.CardNumberHash,
				tx.Amount,
				time.Now(),
				)
				
				if err != nil {
					logger.Error("Failed to insert into ClickHouse", "ERROR", err, "transaction_id", tx.ID)
					//TODO: РЕализовать логику повторных попыток
					return
				}

				logger.Info("транзакция успешно обработана", "transaction_id", tx.ID, "amount=%.2f", tx.Amount, "is_fraudulent", result.IsFraudulent)

			})

			// Commit offsets after successfully processing a batch of messages
			if err := consumerClient.CommitUncommittedOffsets(ctx); err != nil {
				logger.Error("error committing offsets", "error", err)
			}
			
		}
	}

	logger.Info("anti-fraud analyzer останавливается...")
}

// sendToDLQ sends the original malformed message to the Dead-Letter Queue.
func sendToDLQ(p *kgo.Client, originalRecord *kgo.Record, errorType, errorString string) {
	dlqRecord := &kgo.Record{
		Topic: dlqTopic,
		Value: originalRecord.Value,
		Key:   originalRecord.Key,
		// Add headers with metadata about the failure for easier debugging.
		Headers: []kgo.RecordHeader{
			{Key: "error_type", Value: []byte(errorType)},
			{Key: "error_string", Value: []byte(errorString)},
			{Key: "original_topic", Value: []byte(originalRecord.Topic)},
		},
	}
	// Sending asynchronously with a callback
	p.Produce(context.Background(), dlqRecord, func(r *kgo.Record, err error) {
		if err != nil {
			// Критическая ошибка: потеря сообщения в DLQ недопустима.			
			fmt.Fprintf(os.Stderr, "FATAL: Не удалось отправить сообщение в DLQ: %v\n", err)
		}
	})
}
