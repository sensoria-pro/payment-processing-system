// cmd/anti-fraud-analyzer/main.go

package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/redis/go-redis/v9"
	
	"payment-processing-system/internal/antifraud"
	"payment-processing-system/internal/config"
	"payment-processing-system/internal/core/domain"
)

// dlqTopic is the name of our Dead-Letter Queue topic.
var dlqTopic = "transactions.created.dlq"

func main() {
	// --- Configuration Setup ---
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Load configuration from environment variables with sensible defaults for local development.
	kafkaBootstrapServers := getEnv("KAFKA_BOOTSTRAP_SERVERS", "localhost:9092")
	clickhouseAddr := getEnv("CLICKHOUSE_ADDR", "localhost:9000")
	redisAddr := cfg.Redis.Addr

	// --- Component Initialization ---

	// Kafka Consumer: Reads incoming transaction messages.
	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": kafkaBootstrapServers,
		"group.id":          "anti-fraud-group",
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		log.Fatalf("Failed to create Kafka consumer: %s", err)
	}
	defer consumer.Close()

	// Kafka Producer: Used exclusively for sending messages to the DLQ.
	producer, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": kafkaBootstrapServers})
	if err != nil {
		log.Fatalf("Failed to create Kafka producer: %s", err)
	}
	defer producer.Close()

	// ClickHouse Client: For writing fraud analysis results.
	chConn, err := clickhouse.Open(&clickhouse.Options{Addr: []string{clickhouseAddr}})
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}
	defer chConn.Close()

	// Redis Client: Dependency for our caching rule engine.
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})

	// Fraud Rule Engine: Instantiate our chosen rule engine implementation.
	// Thanks to the interface, we could easily swap this for a different engine.
	ruleEngine := antifraud.NewCachingRuleEngine(rdb, cfg.AntiFraud)

	// --- Application Start ---

	// Subscribe to the main transaction topic.
	consumer.Subscribe("transactions.created", nil)

	// Set up graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("Anti-fraud analyzer started...")

	// Main processing loop.
	run := true
	for run {
		select {
		case <-ctx.Done(): // Exit loop on shutdown signal.
			run = false
		default:
			// Poll Kafka for a new message.
			msg, err := consumer.ReadMessage(5 * time.Second)
			if err != nil {
				// Ignore timeout errors, which are expected when the topic is idle.
				if e, ok := err.(kafka.Error); ok && e.Code() == kafka.ErrTimedOut {
					continue
				}
				log.Printf("Consumer error: %v (%v)\n", err, msg)
				continue
			}

			// Attempt to unmarshal the message into our domain Transaction struct.
			var tx domain.Transaction
			if err := json.Unmarshal(msg.Value, &tx); err != nil {
				log.Printf("Failed to unmarshal message: %v. Sending to DLQ.", err)
				// If unmarshalling fails, send the poison pill message to the DLQ.
				sendToDLQ(producer, msg, "unmarshal_error", err.Error())
				continue
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
				log.Printf("Failed to insert into ClickHouse: %v", err)
				// In a real system, a persistent DB failure might also warrant a retry/DLQ mechanism.
				continue
			}

			log.Printf("Processed transaction %s: amount=%.2f, fraud=%v", tx.ID, tx.Amount, result.IsFraudulent)
		}
	}

	log.Println("Anti-fraud analyzer stopped.")
}

// sendToDLQ sends the original malformed message to the Dead-Letter Queue.
func sendToDLQ(p *kafka.Producer, originalMsg *kafka.Message, errorType, errorString string) {
	dlqMessage := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &dlqTopic, Partition: kafka.PartitionAny},
		Value:          originalMsg.Value,
		Key:            originalMsg.Key,
		// Add headers with metadata about the failure for easier debugging.
		Headers: []kafka.Header{
			{Key: "error_type", Value: []byte(errorType)},
			{Key: "error_string", Value: []byte(errorString)},
		},
	}

	err := p.Produce(dlqMessage, nil)
	if err != nil {
		log.Printf("FATAL: Could not produce to DLQ: %v\n", err)
	}
}

// getEnv is a helper function to read an environment variable or return a fallback value.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}