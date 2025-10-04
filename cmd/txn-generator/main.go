package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"math/rand"

	"github.com/go-faker/faker/v4"
	"github.com/google/uuid"
)

// TODO: Структура запроса, должна совпадать с той, что ожидает payment-gateway
type TransactionRequest struct {
	IdempotencyKey string  `json:"idempotency_key"`
	CardNumber     string  `json:"card_number"`
	ExpiryMonth    int     `json:"expiry_month"`
	ExpiryYear     int     `json:"expiry_year"`
	CVC            string  `json:"cvc"`
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
}

func main() {
	// 1. Setting up flags
	targetURL := flag.String("target", "http://localhost:8080/transaction", "Target URL for sending transactions")
	rps := flag.Int("rps", 20, "Requests per second")
	flag.Parse()

	log.Printf("Starting generator: target=%s, rps=%d\n", *targetURL, *rps)

	// 2. Managing the request frequency via ticker
	ticker := time.NewTicker(time.Second / time.Duration(*rps))
	defer ticker.Stop()

	// 3. Graceful Shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 4. Main loop
	for {
		select {
		case <-ticker.C:
			// Start sending in a goroutine so as not to block the ticker
			go sendRequest(*targetURL)
		case <-ctx.Done():
			log.Println("Shutting down generator...")
			return
		}
	}
}

func sendRequest(url string) {
	// Create a fake request
	reqData := TransactionRequest{
		IdempotencyKey: uuid.New().String(),
		CardNumber:     faker.CCNumber(),
		ExpiryMonth:    12,
		ExpiryYear:     2028,
		CVC:            "123",
		Amount:         float64(rand.Intn(100000)) / 100.0,
		Currency:       "RUB",
	}

	body, err := json.Marshal(reqData)
	if err != nil {
		log.Printf("ERROR: failed to marshal request: %v", err)
		return
	}

	// Sending a request
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("ERROR: failed to send request: %v", err)
		return
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body : %v", err)
		}
	}()

	if resp.StatusCode != http.StatusAccepted {
		log.Printf("WARN: received non-202 status code: %d", resp.StatusCode)
	} else {
		log.Printf("INFO: request sent successfully, status: %d", resp.StatusCode)
	}
}
