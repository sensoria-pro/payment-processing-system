package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiterAdapter is a Redis implementation of the RateLimiterRepository port.
type RateLimiterAdapter struct {
	rdb *redis.Client
}

// NewRateLimiterAdapter creates and tests a new connection to Redis and returns the adapter.
func NewRateLimiterAdapter(addr string) (*RateLimiterAdapter, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	if _, err := rdb.Ping(context.Background()).Result(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RateLimiterAdapter{rdb: rdb}, nil
}

// IsAllowed implements the rate limiting logic using a fixed-window algorithm in Redis.
func (a *RateLimiterAdapter) IsAllowed(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	// Atomically increment the counter for the given key.
	count, err := a.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis INCR failed: %w", err)
	}

	// If this is the first request in the window, set the expiration time.
	if count == 1 {
		if err := a.rdb.Expire(ctx, key, window).Err(); err != nil {
			return false, fmt.Errorf("redis EXPIRE failed: %w", err)
		}
	}

	// Check if the count exceeds the limit.
	return count <= int64(limit), nil
}

// Close gracefully closes the Redis connection.
func (a *RateLimiterAdapter) Close() error {
	return a.rdb.Close()
}
