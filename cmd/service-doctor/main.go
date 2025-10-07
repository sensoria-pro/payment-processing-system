package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"log/slog"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"

	"payment-processing-system/internal/config"
	"payment-processing-system/internal/observability"
)

// Config stores all necessary addresses and DSN for connection
// type Config struct {
// 	GatewayAPI     string
// 	AntiFraudAPI   string // We will check the TCP connection, because there is no health endpoint.
// 	AlerterAPI     string
// 	PostgresDSN    string
// 	RedisAddr      string
// 	KafkaBrokers   string
// 	ClickhouseAddr string
// 	KeycloakAddr   string
// 	//VaultAddr      string
// 	OpaAddr        string
// }

// Check describes one diagnostic check
type Check struct {
	Name     string
	Func     func(ctx context.Context) error
	Status   string
	Error    error
	Duration time.Duration
}

// loadConfig reads configuration from environment variables with fallbacks
// func loadConfig() *Config {
// 	return &Config{
// 		GatewayAPI:     getEnv("GATEWAY_API_URL", "http://localhost:8080"),
// 		AntiFraudAPI:   getEnv("ANTIFRAUD_API_URL", "localhost:8082"),
// 		AlerterAPI:     getEnv("ALERTER_API_URL", "http://localhost:8081"),
// 		PostgresDSN:    getEnv("POSTGRES_DSN", "postgres://user:password@localhost:5432/transactionsdb?sslmode=disable"),
// 		RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
// 		KafkaBrokers:   getEnv("KAFKA_BROKERS", "localhost:9092"),
// 		ClickhouseAddr: getEnv("CLICKHOUSE_ADDR", "localhost:9000"),
// 		KeycloakAddr:   getEnv("KEYCLOAK_ADDR", "http://localhost:8888"),
// 		//VaultAddr:      getEnv("VAULT_ADDR", "http://localhost:8200"),
// 		OpaAddr:        getEnv("OPA_ADDR", "http://localhost:8181"),
// 	}
// }

func main() {
	logger := observability.SetupLogger("development")
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		logger.Error("не удалось загрузить конфигурацию", "ERROR", err)
		os.Exit(1)
	}
	// Формируем список проверок, используя данные из config.yaml
	checks := []Check{
		{Name: "Payment Gateway", Func: func(ctx context.Context) error {
			return checkHTTPHealth(ctx, cfg.Server.Port+"/metrics", logger)
		}},
		{Name: "PostgreSQL", Func: func(ctx context.Context) error {
			return checkPostgres(ctx, cfg.Postgres.DSN, logger)
		}},
		{Name: "Redis", Func: func(ctx context.Context) error {
			return checkRedis(ctx, cfg.Redis.Addr, logger)
		}},
		{Name: "Kafka Cluster", Func: func(ctx context.Context) error {
			return checkKafka(ctx, strings.Split(cfg.Kafka.BootstrapServers, ","), logger)
		}},
		{Name: "ClickHouse", Func: func(ctx context.Context) error {
			return checkClickHouse(ctx, cfg.ClickHouse.Addr, logger)
		}},
		{Name: "Keycloak", Func: func(ctx context.Context) error {
			return checkHTTPHealth(ctx, cfg.OIDC.URL+"/health/ready", logger)
		}},
		{Name: "Open Policy Agent", Func: func(ctx context.Context) error {
			return checkHTTPHealth(ctx, cfg.OPA.URL+"/health", logger)
		}},
		// Добавляем проверку для других наших сервисов, если у них есть health-check
		// {Name: "Anti-Fraud Analyzer", Func: func(ctx context.Context) error { return checkHTTPHealth(ctx, "http://localhost:XXXX/healthz") }},
		// {Name: "Alerter Service", Func: func(ctx context.Context) error { return checkHTTPHealth(ctx, "http://localhost:8081/healthz") }},
	}

	// checks := []Check{
	// 	{Name: "Payment Gateway", Func: checkHTTPHealth("/metrics")},
	// 	{Name: "Anti-Fraud Analyzer", Func: checkTCPHealth},
	// 	{Name: "Alerter Service", Func: checkHTTPHealth("/alert")},
	// 	{Name: "PostgreSQL", Func: checkPostgres},
	// 	{Name: "Redis", Func: checkRedis},
	// 	{Name: "Kafka Cluster", Func: checkKafka},
	// 	{Name: "ClickHouse", Func: checkClickHouse},
	// 	{Name: "Keycloak", Func: checkHTTPHealth("/health/ready")},
	// 	{Name: "HashiCorp Vault", Func: checkHTTPHealth("/v1/sys/health")},
	// 	{Name: "Open Policy Agent", Func: checkHTTPHealth("/health")},
	// }

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fmt.Println("🩺 Запуск комплексной диагностики системы...")

	for i := range checks {
		wg.Add(1)
		go func(c *Check) {
			defer wg.Done()
			start := time.Now()
			c.Error = c.Func(ctx) // We pass the config to the verification function
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
	hasErrors := false
	for _, c := range checks {
		if c.Error == nil {
			fmt.Printf("[%s] %-25s (время %v)\n", c.Status, c.Name, c.Duration.Round(time.Millisecond))
		} else {
			hasErrors = true
			fmt.Printf("[%s] %-25s (время %v) - Ошибка: %v\n", c.Status, c.Name, c.Duration.Round(time.Millisecond), c.Error)
		}
	}

	if hasErrors {
		fmt.Println("\nДиагностика выявила проблемы.")
		os.Exit(1)
	}
	fmt.Println("\nВсе системы в норме!")

}

// --- Functions for checks ---

func checkHTTPHealth(ctx context.Context, url string, logger *slog.Logger) error {
	// Добавляем http://, если его нет
	if !strings.HasPrefix(url, "http") {
		url = "http://" + url
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Error("не удалось закрыть Http соединение", "ERROR", err)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("некорректный статус: %s", resp.Status)
	}
	return nil
}


// func checkHTTPHealth(path string) func(context.Context, *Config) error {
// 	return func(ctx context.Context, cfg *Config) error {
// 		// Determine URL based on check name
// 		var url string
// 		switch path {
// 		case "/metrics":
// 			url = cfg.GatewayAPI + path
// 		case "/alert":
// 			url = cfg.AlerterAPI + path
// 		case "/health/ready":
// 			url = cfg.KeycloakAddr + path
// 		case "/v1/sys/health":
// 			//url = cfg.VaultAddr + path
// 		case "/health":
// 			url = cfg.OpaAddr + path
// 		default:
// 			return fmt.Errorf("unknown http path: %s", path)
// 		}

// 		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
// 		if err != nil {
// 			return err
// 		}

// 		resp, err := http.DefaultClient.Do(req)
// 		if err != nil {
// 			return err
// 		}
		
// 			defer func() {
// 				if err := resp.Body.Close(); err != nil {
// 					fmt.Printf("error closing HTTP response body: %v", err)
// 				}
// 			}()

// 		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
// 			return fmt.Errorf("bad status: %s", resp.Status)
// 		}
// 		return nil
// 	}
// }

// func checkTCPHealth(ctx context.Context, cfg *Config) error {
// 	d := net.Dialer{Timeout: 5 * time.Second}
// 	conn, err := d.DialContext(ctx, "tcp", cfg.AntiFraudAPI)
// 	if err != nil {
// 		return err
// 	}

// 	defer func() {
// 		if err := conn.Close(); err != nil {
// 			fmt.Printf("error closing TCP connection: %v", err)
// 		}
// 	}()
	
// 	return nil
// }


func checkPostgres(ctx context.Context, dsn string, logger *slog.Logger) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}

	defer func() {
		if err := conn.Close(ctx); err != nil {
			logger.Error("не удалось закрыть соединение Postgres", "ERROR", err)
		}
	}()
	return conn.Ping(ctx)
}

func checkRedis(ctx context.Context, addr string, logger *slog.Logger) error {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	defer func() {
		if err := rdb.Close(); err != nil {
			logger.Error("не удалось закрыть Redis", "ERROR", err)
		}
	}()
	return rdb.Ping(ctx).Err()
}

// Improved Kafka Validation via Admin Client
func checkKafka(ctx context.Context, brokers []string, logger *slog.Logger) error {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.DialTimeout(5*time.Second),
	)
	if err != nil {
		return err
	}
	defer client.Close()
	

	// Ping проверяет, что мы можем подключиться к брокерам
	return client.Ping(ctx)
}

func checkClickHouse(ctx context.Context, addr string, logger *slog.Logger) error {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			// Предполагаем, что для health-check не нужен пароль,
			//TODO: в проде реализовать
			Database: "default",
			Username: "default",
		},
	})
	if err != nil {
		return err
	}
	//defer conn.Close()
	defer func() {
		if err := conn.Close(); err != nil {
			logger.Error("не удалось закрыть clickhouse", "ERROR", err)
		}
	}()
	return conn.Ping(ctx)
}