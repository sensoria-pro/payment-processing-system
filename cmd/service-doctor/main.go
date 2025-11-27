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
			return checkHTTPHealth(ctx, cfg.Server.Port+"/healthz", logger)
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
			return checkClickHouse(ctx, cfg.ClickHouse, logger)
		}},
		{Name: "Keycloak", Func: func(ctx context.Context) error {
			return checkHTTPHealth(ctx, cfg.OIDC.URL+"/health/ready", logger)
		}},
		// {Name: "HashiCorp Vault", Func: func(ctx context.Context) error {
		// 	return checkHTTPHealth(ctx, "http://localhost:8200/v1/sys/health", logger)
		// }},
		{Name: "Open Policy Agent", Func: func(ctx context.Context) error {
			return checkHTTPHealth(ctx, cfg.OPA.URL+"/health", logger)
		}},
		// Добавляем проверку для других наших сервисов, если у них есть health-check
		{Name: "Anti-Fraud Analyzer", Func: func(ctx context.Context) error { return checkHTTPHealth(ctx, "http://localhost:8082/healthz", logger) }},
		{Name: "Alerter Service", Func: func(ctx context.Context) error { return checkHTTPHealth(ctx, "http://localhost:8081/healthz", logger) }},
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

func checkClickHouse(ctx context.Context, cfg config.ClickHouseConfig, logger *slog.Logger) error {
	if cfg.Addr == "" {
		return fmt.Errorf("адрес ClickHouse не указан в конфигурации")
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.Addr},
		Auth: clickhouse.Auth{
			// Используем данные из конфигурации
			Database: cfg.Database,
			Username: cfg.User,
			Password: cfg.Password,
		},
		DialTimeout: 5 * time.Second, // Добавляем таймаут для надёжности
	})
	if err != nil {
		return fmt.Errorf("не удалось открыть соединение: %w", err)
	}

	defer func() {
		if err := conn.Close(); err != nil {
			logger.Error("не удалось закрыть соединение с ClickHouse", "ERROR", err)
		}
	}()

	// Ping проверяет, что соединение установлено и аутентификация прошла успешно
	return conn.Ping(ctx)
}