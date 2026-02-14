package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

// Config represents the application configuration
type Config struct {
	Upstream   string      `yaml:"upstream"`
	Port       int         `yaml:"port"`
	Intercepts []Intercept `yaml:"intercepts"`
	Logging    Logging     `yaml:"logging,omitempty"`
	Storage    Storage     `yaml:"storage,omitempty"`
}

// Intercept represents an interceptor configuration
type Intercept struct {
	Endpoint    string `yaml:"endpoint"`
	Interceptor string `yaml:"interceptor"`
}

// Storage represents the storage configuration
type Storage struct {
	Type     string          `yaml:"type"`
	Postgres *PostgresConfig `yaml:"postgres,omitempty"`
}

// PostgresConfig represents the PostgreSQL configuration
type PostgresConfig struct {
	DSN string `yaml:"dsn"`
}

// Logging represents the logging configuration
type Logging struct {
	Format string `yaml:"format,omitempty"`
}

// LoadConfig loads the configuration from a YAML file
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	// Expand environment variables
	expandedData := expandEnv(string(data))

	var config Config
	err = yaml.Unmarshal([]byte(expandedData), &config)
	if err != nil {
		return nil, err
	}

	// Set default logging format if not specified
	if config.Logging.Format == "" {
		config.Logging.Format = "text"
	}

	return &config, nil
}

// expandEnv expands environment variables in the form ${VAR} or ${VAR:-default}
func expandEnv(s string) string {
	return os.Expand(s, func(key string) string {
		if strings.Contains(key, ":-") {
			parts := strings.SplitN(key, ":-", 2)
			val, ok := os.LookupEnv(parts[0])
			if ok {
				return val
			}
			return parts[1]
		}
		return os.Getenv(key)
	})
}
