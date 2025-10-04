package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Config stores all necessary addresses and DSN for connection
type Config struct {
	GatewayAPI     string
	AntiFraudAPI   string // We will check the TCP connection, because there is no health endpoint.
	AlerterAPI     string
	PostgresDSN    string
	RedisAddr      string
	KafkaBrokers   string
	ClickhouseAddr string
	KeycloakAddr   string
	//VaultAddr      string
	OpaAddr        string
}

// Check describes one diagnostic check
type Check struct {
	Name     string
	Func     func(context.Context, *Config) error
	Status   string
	Error    error
	Duration time.Duration
}

// loadConfig reads configuration from environment variables with fallbacks
func loadConfig() *Config {
	return &Config{
		GatewayAPI:     getEnv("GATEWAY_API_URL", "http://localhost:8080"),
		AntiFraudAPI:   getEnv("ANTIFRAUD_API_URL", "localhost:8082"),
		AlerterAPI:     getEnv("ALERTER_API_URL", "http://localhost:8081"),
		PostgresDSN:    getEnv("POSTGRES_DSN", "postgres://user:password@localhost:5432/transactionsdb?sslmode=disable"),
		RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
		KafkaBrokers:   getEnv("KAFKA_BROKERS", "localhost:9092"),
		ClickhouseAddr: getEnv("CLICKHOUSE_ADDR", "localhost:9000"),
		KeycloakAddr:   getEnv("KEYCLOAK_ADDR", "http://localhost:8888"),
		//VaultAddr:      getEnv("VAULT_ADDR", "http://localhost:8200"),
		OpaAddr:        getEnv("OPA_ADDR", "http://localhost:8181"),
	}
}

func main() {
	cfg := loadConfig()

	// We are expanding the list of checks to include all components of our system
	checks := []Check{
		{Name: "Payment Gateway", Func: checkHTTPHealth("/metrics")},
		{Name: "Anti-Fraud Analyzer", Func: checkTCPHealth},
		{Name: "Alerter Service", Func: checkHTTPHealth("/alert")},
		{Name: "PostgreSQL", Func: checkPostgres},
		{Name: "Redis", Func: checkRedis},
		{Name: "Kafka Cluster", Func: checkKafka},
		{Name: "ClickHouse", Func: checkClickHouse},
		{Name: "Keycloak", Func: checkHTTPHealth("/health/ready")},
		{Name: "HashiCorp Vault", Func: checkHTTPHealth("/v1/sys/health")},
		{Name: "Open Policy Agent", Func: checkHTTPHealth("/health")},
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fmt.Println("🩺 Запуск комплексной диагностики системы...")

	for i := range checks {
		wg.Add(1)
		go func(c *Check) {
			defer wg.Done()
			start := time.Now()
			c.Error = c.Func(ctx, cfg) // We pass the config to the verification function
			c.Duration = time.Since(start)
			if c.Error == nil {
				c.Status = "✅ OK"
			} else {
				c.Status = "❌ FAILED"
			}
		}(&checks[i])
	}

	wg.Wait()

	fmt.Println("\n--- Отчёт по диагностике ---")
	for _, c := range checks {
		if c.Error == nil {
			fmt.Printf("[%s] %-25s (время %v)\n", c.Status, c.Name, c.Duration.Round(time.Millisecond))
		} else {
			fmt.Printf("[%s] %-25s (время %v) - Ошибка: %v\n", c.Status, c.Name, c.Duration.Round(time.Millisecond), c.Error)
		}
	}
}

// --- Functions for checks ---

func checkHTTPHealth(path string) func(context.Context, *Config) error {
	return func(ctx context.Context, cfg *Config) error {
		// Determine URL based on check name
		var url string
		switch path {
		case "/metrics":
			url = cfg.GatewayAPI + path
		case "/alert":
			url = cfg.AlerterAPI + path
		case "/health/ready":
			url = cfg.KeycloakAddr + path
		case "/v1/sys/health":
			//url = cfg.VaultAddr + path
		case "/health":
			url = cfg.OpaAddr + path
		default:
			return fmt.Errorf("unknown http path: %s", path)
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("bad status: %s", resp.Status)
		}
		return nil
	}
}

func checkTCPHealth(ctx context.Context, cfg *Config) error {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", cfg.AntiFraudAPI)
	if err != nil {
		return err
	}
	return conn.Close()
}

func checkPostgres(ctx context.Context, cfg *Config) error {
	conn, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Ping(ctx)
}

func checkRedis(ctx context.Context, cfg *Config) error {
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	return rdb.Ping(ctx).Err()
}

// Improved Kafka Validation via Admin Client
func checkKafka(ctx context.Context, cfg *Config) error {
	adminClient, err := kafka.NewAdminClient(&kafka.ConfigMap{"bootstrap.servers": cfg.KafkaBrokers})
	if err != nil {
		return err
	}
	defer adminClient.Close()

	// Requesting cluster metadata with a timeout
	_, err = adminClient.GetMetadata(nil, false, 5000)
	return err
}

func checkClickHouse(ctx context.Context, cfg *Config) error {
	conn, err := clickhouse.Open(&clickhouse.Options{Addr: []string{cfg.ClickhouseAddr}})
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Ping(ctx)
}

// getEnv - helper function for reading environment variables
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
