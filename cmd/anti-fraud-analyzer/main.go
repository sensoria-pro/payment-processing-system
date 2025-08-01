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
)

//The structure we get from Kafka
type Transaction struct {
	ID             string    `json:"ID"`
	CardNumberHash string    `json:"CardNumberHash"`
	Amount         float64   `json:"Amount"`
	Currency       string    `json:"Currency"`
	CreatedAt      time.Time `json:"CreatedAt"`
}

func main() {
	// Get settings from environment variables
	kafkaBootstrapServers := os.Getenv("KAFKA_BOOTSTRAP_SERVERS")
	if kafkaBootstrapServers == "" {
		kafkaBootstrapServers = "localhost:9092"
	}

	clickhouseAddr := os.Getenv("CLICKHOUSE_ADDR")
	if clickhouseAddr == "" {
		clickhouseAddr = "localhost:9000"
	}

	// Kafka Consumer
	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": kafkaBootstrapServers,
		"group.id":          "anti-fraud-group",
		"auto.offset.reset": "earliest",
	})
	if err != nil {
		log.Fatalf("Failed to create consumer: %s", err)
	}
	defer consumer.Close()

	// ClickHouse Client
	chConn, err := clickhouse.Open(&clickhouse.Options{Addr: []string{clickhouseAddr}})
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}
	defer chConn.Close()

	// Subscribe to topic Kafka Consumer
	consumer.Subscribe("transactions.created", nil)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("Anti-fraud analyzer started...")

	// Main processing cycle
	run := true
	for run {
		select {
		case <-ctx.Done():
			run = false
		default:
			msg, err := consumer.ReadMessage(5 * time.Second)
			if err != nil {
				// Ignore timeOut errors
				if e, ok := err.(kafka.Error); ok && e.Code() == kafka.ErrTimedOut {
					continue
				}
				log.Printf("Consumer error: %v (%v)\n", err, msg)
				continue
			}

			var tx Transaction
			if err := json.Unmarshal(msg.Value, &tx); err != nil {
				log.Printf("Failed to unmarshal message: %v", err)
				// TODO: Отправить в DLQ
				continue
			}

			// TODO: Правило: считать фродом транзакции на сумму > 1000
			isFraud := tx.Amount > 1000.0
			reason := ""
			if isFraud {
				reason = "Amount exceeds threshold"
			}

			// save record in ClickHouse
			err = chConn.Exec(ctx, `
				INSERT INTO default.fraud_reports (transaction_id, is_fraudulent, reason, card_hash, amount, processed_at) VALUES (?, ?, ?, ?, ?, ?)`,
				tx.ID,
				isFraud,
				reason,
				tx.CardNumberHash,
				tx.Amount,
				time.Now(),
			)
			if err != nil {
				log.Printf("Failed to insert into ClickHouse: %v", err)
				continue
			}

			log.Printf("Processed transaction %s: amount=%.2f, fraud=%v", tx.ID, tx.Amount, isFraud)
		}
	}

	log.Println("Anti-fraud analyzer stopped.")
}
