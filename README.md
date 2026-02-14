# LLM Monitor

LLM Monitor is a Go-based proxy server designed to intercept, monitor, and log interactions with Large Language Models (LLMs). It is specifically tailored for [Ollama](https://ollama.com/), but its modular architecture allows for easy extension to other LLM providers.

## Features

- **Transparent Proxying**: Forwards requests to an upstream LLM server (like Ollama).
- **Request/Response Interception**: Intercept and modify requests and responses.
- **Streaming Support**: Fully supports streaming responses (`stream: true`) common in LLM APIs.
- **Persistence**: Logs conversations and messages to a PostgreSQL database.
- **Modular Interceptors**:
    - `OllamaChatInterceptor`: Intercepts `/api/chat` requests and logs messages.
    - `OllamaGenerateInterceptor`: Intercepts `/api/generate` requests and logs prompts.
    - `LoggingInterceptor`: Simple logging of requests.
    - `CustomInterceptor` & `SimpleInterceptor`: Examples for custom implementations.
- **Configurable**: Easy setup using YAML configuration and environment variables.
- **Docker Ready**: Includes `Dockerfile` and `docker-compose.yml` for quick deployment.

## Prerequisites

- **Go**: 1.25 or later (if running locally).
- **Docker & Docker Compose**: (optional, for containerized deployment).
- **PostgreSQL**: (if persistence is enabled).

## Getting Started

### Using Docker Compose (Recommended)

The easiest way to get started is using Docker Compose, which sets up the proxy and a PostgreSQL database.

1. Clone the repository.
2. Run the services:
   ```bash
   docker-compose up -d
   ```
3. The proxy will be available at `http://localhost:8080`.

### Running Locally

1. Install dependencies:
   ```bash
   go mod download
   ```
2. Set up your environment variables or modify `configs/config.yaml`.
3. Run the application:
   ```bash
   go run cmd/main.go -c configs/config.yaml
   ```

## Configuration

The application is configured via a YAML file (default `config.yaml`). You can use environment variables within the YAML file using the `${VAR:-default}` syntax.

### Example `config.yaml`

```yaml
logging:
  format: "json"  # "json" or "text"
port: 8080
upstream: "${UPSTREAM_URL:-http://localhost:11434}"
intercepts:
  - endpoint: "/api/generate"
    interceptor: "OllamaGenerateInterceptor"
  - endpoint: "/api/chat"
    interceptor: "OllamaChatInterceptor"

storage:
  type: "postgres"
  postgres:
    dsn: "postgres://${DB_USER:-user}:${DB_PASSWORD:-password}@${DB_HOST:-localhost}:${DB_PORT:-5432}/${DB_NAME:-llm_monitor}?sslmode=disable"
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `UPSTREAM_URL` | The URL of the LLM server to proxy to | `http://localhost:11434` |
| `DB_USER` | PostgreSQL user | `user` |
| `DB_PASSWORD` | PostgreSQL password | `password` |
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_NAME` | PostgreSQL database name | `llm_monitor` |

## Interceptors

Interceptors implement the `Interceptor` interface found in `internal/interceptor/interceptor.go`. They can hook into various stages of the request-response lifecycle:

- `RequestInterceptor`: Modify the request before it reaches the upstream.
- `ResponseInterceptor`: Modify the response headers/status.
- `ContentInterceptor`: Modify the full response body (non-streaming).
- `ChunkInterceptor`: Process individual chunks in a streaming response.
- `OnComplete`: Called after the response is fully delivered.

## Database Schema

When using PostgreSQL, the application tracks:
- **Conversations**: High-level containers for a series of messages.
- **Branches**: Support for branching conversations (e.g., retries or different paths).
- **Messages**: The actual content, role, and sequence within a branch.

The schema is automatically initialized when using Docker Compose via `internal/storage/schema.sql`.

## Testing

A `test/test-queries.http` file is provided for use with IDEs like IntelliJ or VS Code (REST Client) to quickly test various endpoints and interceptors.

```bash
# Example test using curl
curl -X POST http://localhost:8080/api/chat \
     -H "Content-Type: application/json" \
     -d '{
       "model": "llama3",
       "messages": [{"role": "user", "content": "Hello!"}],
       "stream": false
     }'
```
