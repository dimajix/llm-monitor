package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// Config holds the application configuration
type Config struct {
	Port       int               `yaml:"port"`
	Upstream   string            `yaml:"upstream"`
	Intercepts []InterceptConfig `yaml:"intercepts"`
}

// InterceptConfig represents a single interceptor configuration
type InterceptConfig struct {
	Endpoint    string `yaml:"endpoint"`
	Interceptor string `yaml:"interceptor"`
}

// LoadConfig loads configuration from YAML file
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	return &config, nil
}
