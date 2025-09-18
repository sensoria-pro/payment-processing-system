package redis

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
)

// NewClient creates and tests a new connection to Redis.
func NewClient(addr string) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	if _, err := rdb.Ping(context.Background()).Result(); err != nil {
		return nil, fmt.Errorf("Failed to connect to Redis: %w", err)
	}

	return rdb, nil
}