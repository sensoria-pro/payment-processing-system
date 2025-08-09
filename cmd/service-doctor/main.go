package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Check struct {
	Name     string
	Func     func(context.Context) error
	Status   string
	Error    error
	Duration time.Duration
}

func main() {
	checks := []Check{
		{Name: "Payment Gateway API", Func: checkGatewayAPI},
		{Name: "PostgreSQL", Func: checkPostgres},
		{Name: "Redis", Func: checkRedis},
		{Name: "Kafka", Func: checkKafka},
		{Name: "ClickHouse", Func: checkClickHouse},
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Println("🩺 Running system diagnostics...")

	for i := range checks {
		wg.Add(1)
		go func(c *Check) {
			defer wg.Done()
			start := time.Now()
			c.Error = c.Func(ctx)
			c.Duration = time.Since(start)
			if c.Error == nil {
				c.Status = "✅ OK"
			} else {
				c.Status = "❌ FAILED"
			}
		}(&checks[i])
	}

	wg.Wait()

	fmt.Println("\n--- Diagnostics Report ---")
	for _, c := range checks {
		if c.Error == nil {
			fmt.Printf("[%s] %-25s (took %v)\n", c.Status, c.Name, c.Duration.Round(time.Millisecond))
		} else {
			fmt.Printf("[%s] %-25s (took %v) - Error: %v\n", c.Status, c.Name, c.Duration.Round(time.Millisecond), c.Error)
		}
	}
}

//TODO: функции-заглушки для проверок
func checkGatewayAPI(ctx context.Context) error { _, err := http.Get("http://localhost:8080/metrics"); return err }
func checkPostgres(ctx context.Context) error { conn, err := pgxpool.New(ctx, "postgres://user:password@localhost:5432/transactionsdb?sslmode=disable"); if err != nil {return err}; defer conn.Close(); return conn.Ping(ctx) }
func checkRedis(ctx context.Context) error { rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"}); return rdb.Ping(ctx).Err() }
func checkKafka(ctx context.Context) error { p, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": "localhost:9092"}); if err != nil {return err}; p.Close(); return nil }
func checkClickHouse(ctx context.Context) error { conn, err := clickhouse.Open(&clickhouse.Options{Addr:[]string{"localhost:9000"}}); if err != nil {return err}; return conn.Ping(ctx)}