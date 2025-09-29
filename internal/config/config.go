package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
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
		Port     string `yaml:"port"`
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
		JWTSecret string `yaml:"jwt_secret"`
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
	if config.AntiFraud.AmountThreshold == 0 {
		config.AntiFraud.AmountThreshold = 1000.0
	}
	if config.AntiFraud.FrequencyThreshold == 0 {
		config.AntiFraud.FrequencyThreshold = 3
	}
	if config.AntiFraud.FrequencyWindowSeconds == 0 {
		config.AntiFraud.FrequencyWindowSeconds = 60
	}
	return config, nil

}