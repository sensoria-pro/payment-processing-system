package antifraud

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"payment-processing-system/internal/core/domain"
)

// ExternalServiceRuleEngine - calls an external API to make decisions.
type ExternalServiceRuleEngine struct {
	client  *http.Client
	scorerURL string
}

// NewExternalServiceRuleEngine - creates a new engine.
func NewExternalServiceRuleEngine(scorerURL string) *ExternalServiceRuleEngine {
	return &ExternalServiceRuleEngine{
		// Create http.Client with timeouts for fault tolerance
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		scorerURL: scorerURL,
	}
}

// CheckTransaction  - implements the verification logic through an external call.
func (e *ExternalServiceRuleEngine) CheckTransaction(tx domain.Transaction) domain.FraudResult {
	// 1. Packing the transaction into JSON for sending
	requestBody, err := json.Marshal(tx)
	if err != nil {
		log.Printf("ERROR: Failed to marshal transaction for external service: %v", err)
		return domain.FraudResult{IsFraudulent: false, Reason: ""}
	}

	// 2. Create and send an HTTP request
	req, err := http.NewRequestWithContext(context.Background(), "POST", e.scorerURL, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Printf("ERROR: Failed to create request for external service: %v", err)
		return domain.FraudResult{IsFraudulent: false, Reason: ""}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		log.Printf("ERROR: External fraud scoring service call failed: %v", err)
		return domain.FraudResult{IsFraudulent: false, Reason: ""}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("ERROR: External fraud scoring service returned non-200 status: %s", resp.Status)
		return domain.FraudResult{IsFraudulent: false, Reason: ""}
	}

	// 3. Unpacking the answer
	var result domain.FraudResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("ERROR: Failed to decode response from external service: %v", err)
		return domain.FraudResult{IsFraudulent: false, Reason: ""}
	}

	return result
}