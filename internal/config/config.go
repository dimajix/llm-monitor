package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// Config represents the application configuration
type Config struct {
	Upstream   string      `yaml:"upstream"`
	Port       int         `yaml:"port"`
	Intercepts []Intercept `yaml:"intercepts"`
	Logging    Logging     `yaml:"logging,omitempty"`
}

// Intercept represents an interceptor configuration
type Intercept struct {
	Endpoint    string `yaml:"endpoint"`
	Interceptor string `yaml:"interceptor"`
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

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	// Set default logging format if not specified
	if config.Logging.Format == "" {
		config.Logging.Format = "text"
	}

	return &config, nil
}
