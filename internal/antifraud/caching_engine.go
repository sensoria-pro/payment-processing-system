package antifraud

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"payment-processing-system/internal/config"
	"payment-processing-system/internal/core/domain"
)

// CachingRuleEngine implements the FraudRuleEngine interface using Redis for stateful checks.
type CachingRuleEngine struct {
	rdb  *redis.Client
	cfg  config.AntiFraudConfig
}
// NewCachingRuleEngine creates a new engine connected to Redis.
func NewCachingRuleEngine(rdb *redis.Client, cfg config.AntiFraudConfig) *CachingRuleEngine {
	return &CachingRuleEngine{
		rdb: rdb,
		cfg: cfg,
	}
}

// CheckTransaction implements the fraud checking logic using Redis.
func (e *CachingRuleEngine) CheckTransaction(tx domain.Transaction) domain.FraudResult {
	ctx := context.Background()

	// Rule 1: Transaction amount exceeds a simple threshold.  (TODO: default < 1000)
	amountThreshold := e.cfg.AmountThreshold
	if tx.Amount > amountThreshold {
		return domain.FraudResult{IsFraudulent: true, Reason: "Amount exceeds threshold"}
	}

	// Rule 2: More than 3 transactions from a single card within a 1-minute window.
	key := fmt.Sprintf("card_tx_count:%s", tx.CardNumberHash)

	// Atomically increment the counter for this card hash.
	count, err := e.rdb.Incr(ctx, key).Result()
	if err != nil {
		log.Printf("ERROR: Redis INCR failed: %v", err)
		return domain.FraudResult{}
	}

	if count == 1 {
		// Set the lifetime of the key from the config (TODO: default - 60 second)
		freqWindowSec := int64(e.cfg.FrequencyWindowSeconds)
		ttl := time.Duration(freqWindowSec) * time.Second
		e.rdb.Expire(ctx, key, ttl)
	}
	// Set the lifetime of the key from the config (TODO: default Threshold - 3 transactions)
	freqThreshold := int64(e.cfg.FrequencyThreshold)

	if count > freqThreshold {
		reason := fmt.Sprintf(
			"High frequency: %d transactions in %d seconds", 
			count, 
			e.cfg.FrequencyWindowSeconds,
		)
		return domain.FraudResult{IsFraudulent: true, Reason: reason}
	}

	return domain.FraudResult{}
}