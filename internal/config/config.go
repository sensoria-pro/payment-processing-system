package config

import (
	"fmt"
	"os"
	"gopkg.in/yaml.v3"
)
// AntiFraudConfig stores parameters for the rules engine.
type AntiFraudConfig struct {
	AmountThreshold        float64 `yaml:"amount_threshold"`
	FrequencyThreshold     int     `yaml:"frequency_threshold"`
	FrequencyWindowSeconds int     `yaml:"frequency_window_seconds"`
}

type Config struct {
	App struct {
		Env string `yaml:"env"`
	} `yaml:"app"`
	Server struct {
		Port string `yaml:"port"`
	} `yaml:"server"`
	Postgres struct {
		DSN string `yaml:"dsn"`
	} `yaml:"postgres"`
	Kafka struct {
		BootstrapServers string `yaml:"bootstrap_servers"`
		Topic            string `yaml:"topic"`
	} `yaml:"kafka"`
	Redis struct {
		Addr string `yaml:"addr"`
	} `yaml:"redis"`
	Jaeger struct {
		Port string `yaml:"port"`
		PortGrpc string `yaml:"port_grpc"`
	} `yaml:"jaeger"`
	OIDC struct {
		URL      string `yaml:"url"`
		ClientID string `yaml:"client_id"`
	} `yaml:"oidc"`
	OPA struct {
		URL string `yaml:"url"`
	} `yaml:"opa"`
	JWT struct {
		JWT_secret string `yaml:"jwt_secret"`
	} `yaml:"jwt"`
	AntiFraud AntiFraudConfig `yaml:"anti_fraud"`
}

func Load(configPath string) (*Config, error) {
	config := &Config{}

	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// First, we substitute environment variables into the raw YAML file.
	expandedFile := os.ExpandEnv(string(file))

	err = yaml.Unmarshal([]byte(expandedFile), config)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	return config, nil

	// Replace environment variables in the configuration
	// config.ServerPort = expandEnv(config.ServerPort)
	// config.Postgres.DSN = expandEnv(config.Postgres.DSN)
	// config.Kafka.BootstrapServers = expandEnv(config.Kafka.BootstrapServers)
	// config.Kafka.Topic = expandEnv(config.Kafka.Topic)
	// config.Redis.Addr = expandEnv(config.Redis.Addr)
	// return config, nil
}

// expandEnv replaces environment variables in a string
// func expandEnv(s string) string {
// 	return os.Expand(s, func(key string) string {
// 		if val := os.Getenv(key); val != "" {
// 			return val
// 		}
// 		// If the variable is not found, return the original string
// 		return "${" + key + "}"
// 	})
// }
