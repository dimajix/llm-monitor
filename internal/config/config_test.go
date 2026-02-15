package config

import (
	"os"
	"testing"
)

func TestLoadConfig_EnvSubstitution(t *testing.T) {
	// Create a temporary config file with env vars
	content := `
port: ${PORT:-9090}
upstream:
  url: ${UPSTREAM_URL}
storage:
  type: "postgres"
  postgres:
    dsn: "postgres://${DB_USER}:${DB_PASS}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"
`
	tmpfile, err := os.CreateTemp("", "config_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Set environment variables
	os.Setenv("UPSTREAM_URL", "http://ollama:11434")
	os.Setenv("DB_USER", "admin")
	os.Setenv("DB_PASS", "secret")
	os.Setenv("DB_HOST", "db.example.com")
	os.Setenv("DB_PORT", "5432")
	os.Setenv("DB_NAME", "mydb")
	defer func() {
		os.Unsetenv("UPSTREAM_URL")
		os.Unsetenv("DB_USER")
		os.Unsetenv("DB_PASS")
		os.Unsetenv("DB_HOST")
		os.Unsetenv("DB_PORT")
		os.Unsetenv("DB_NAME")
	}()

	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Port != 9090 {
		t.Errorf("Expected Port 9090, got %d", cfg.Port)
	}
	if cfg.Upstream.URL != "http://ollama:11434" {
		t.Errorf("Expected Upstream http://ollama:11434, got %s", cfg.Upstream.URL)
	}
	expectedDSN := "postgres://admin:secret@db.example.com:5432/mydb?sslmode=disable"
	if cfg.Storage.Postgres.DSN != expectedDSN {
		t.Errorf("Expected DSN %s, got %s", expectedDSN, cfg.Storage.Postgres.DSN)
	}
}

func TestLoadConfig_EnvDefaults(t *testing.T) {
	// Create a temporary config file with env vars and defaults
	content := `
port: ${PORT:-8081}
upstream:
  url: ${UPSTREAM_URL:-http://localhost:11434}
`
	tmpfile, err := os.CreateTemp("", "config_defaults_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Ensure env vars are NOT set
	os.Unsetenv("PORT")
	os.Unsetenv("UPSTREAM_URL")

	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Port != 8081 {
		t.Errorf("Expected Port 8081 (default), got %d", cfg.Port)
	}
	if cfg.Upstream.URL != "http://localhost:11434" {
		t.Errorf("Expected Upstream http://localhost:11434 (default), got %s", cfg.Upstream.URL)
	}
}

func TestLoadConfig_Timeout(t *testing.T) {
	content := `
port: 8080
upstream:
  url: http://localhost:11434
  timeout: 60s
storage:
  type: postgres
  timeout: 10s
`
	tmpfile, err := os.CreateTemp("", "config_timeout_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Upstream.Timeout != "60s" {
		t.Errorf("Expected Upstream Timeout 60s, got %s", cfg.Upstream.Timeout)
	}
	if cfg.Storage.Timeout != "10s" {
		t.Errorf("Expected Storage Timeout 10s, got %s", cfg.Storage.Timeout)
	}
}

func TestLoadConfig_Intercepts(t *testing.T) {
	content := `
intercepts:
  - endpoint: "/api/chat"
    method: "POST"
    interceptor: "OllamaChatInterceptor"
  - endpoint: "/api/generate"
    method: "*"
    interceptor: "OllamaGenerateInterceptor"
`
	tmpfile, err := os.CreateTemp("", "config_intercepts_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(cfg.Intercepts) != 2 {
		t.Fatalf("Expected 2 intercepts, got %d", len(cfg.Intercepts))
	}

	if cfg.Intercepts[0].Endpoint != "/api/chat" || cfg.Intercepts[0].Method != "POST" {
		t.Errorf("Unexpected intercept 0: %+v", cfg.Intercepts[0])
	}

	if cfg.Intercepts[1].Endpoint != "/api/generate" || cfg.Intercepts[1].Method != "*" {
		t.Errorf("Unexpected intercept 1: %+v", cfg.Intercepts[1])
	}
}
