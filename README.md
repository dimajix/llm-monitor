# LLM Monitor

LLM Monitor is a Go-based proxy server designed to intercept, monitor, and log interactions with Large Language Models (LLMs). It includes a built-in web interface for viewing and searching conversation history. It is specifically tailored for [Ollama](https://ollama.com/), but its modular architecture allows for easy extension to other LLM providers.

## Features

- **Transparent Proxying**: Forwards requests to an upstream LLM server (like Ollama).
- **Request/Response Interception**: Intercept and modify requests and responses.
- **Streaming Support**: Fully supports streaming responses (`stream: true`) common in LLM APIs.
- **Persistence**: Logs conversations and messages to a PostgreSQL database.
- **Web UI**: Modern, built-in web interface to browse, search, and visualize conversation histories (served by the API binary).
- **Modular Interceptors**:
    - `OpenAIChatInterceptor`: Intercepts `/v1/chat/completions` requests and logs messages in OpenAI format.
    - `OllamaChatInterceptor`: Intercepts `/api/chat` requests and logs messages in Ollama format.
    - `OllamaGenerateInterceptor`: Intercepts `/api/generate` requests and logs prompts.
    - `LoggingInterceptor`: Simple logging of requests.
    - `CustomInterceptor` & `SimpleInterceptor`: Examples for custom implementations.
- **Configurable**: Easy setup using YAML configuration and environment variables.
- **Docker Ready**: Includes `Dockerfile` and `docker-compose.yml` for quick deployment.

## Prerequisites

- **Go**: 1.25 or later (if building locally).
- **Node.js & npm**: (if building the web UI locally).
- **Docker & Docker Compose**: (optional, for containerized deployment).
- **PostgreSQL**: (required for persistence and API/UI functionality).

## Getting Started

### Using Docker Compose (Recommended)

The easiest way to get started is using Docker Compose, which sets up the proxy, the API/UI server, and a PostgreSQL database.

1. **Clone the repository**.
2. **Configure Upstream (Optional)**:
   By default, it proxies to `http://localhost:11434` (Ollama). You can change this in `docker-compose.yml` or via the `UPSTREAM_URL` environment variable.
3. **Run the services**:
   ```bash
   docker compose up -d
   ```
4. **Access the services**:
    - **Proxy**: `http://localhost:8080`
    - **API / Web UI**: `http://localhost:8081`

The first time you run `docker compose up`, the database will be initialized automatically.

### Accessing the Web UI

The Web UI provides a clean interface to explore your conversation history.
1. Open your browser and navigate to `http://localhost:8081`.
2. You will see a list of recent conversations.
3. Click on a conversation to view the full message history, including system prompts, user messages, and assistant responses.
4. Use the search bar to filter conversations by model name or message content.

### Proxying OpenAI Compatible APIs

LLM Monitor supports proxying OpenAI compatible chat completion endpoints. This allows you to monitor traffic from tools and SDKs that use the OpenAI format (like `openai-python`, `langchain`, etc.).

1. **Configure the Interceptor**: Ensure your `config.yaml` has the `OpenAIChatInterceptor` configured for the `/v1/chat/completions` endpoint.
   ```yaml
   proxy:
     intercepts:
       - endpoint: "/v1/chat/completions"
         method: "POST"
         interceptor: "OpenAIChatInterceptor"
   ```
2. **Update your Client**: Point your OpenAI client to the LLM Monitor proxy instead of the original provider.
   ```python
   # Example in Python
   from openai import OpenAI
   client = OpenAI(
       base_url="http://localhost:8080/v1",
       api_key="your-api-key"
   )
   ```
3. **View Logs**: All requests sent through this endpoint will now be captured and visible in the Web UI.

### Proxying Ollama APIs

For Ollama, the following endpoints are supported by default:
- `/api/chat`: Monitored by `OllamaChatInterceptor`.
- `/api/generate`: Monitored by `OllamaGenerateInterceptor`.

Configure your Ollama client or environment variable:
```bash
export OLLAMA_HOST=http://localhost:8080
```

### Building and Running Locally

1. **Build Everything**:
   Use the provided `Makefile` to build both the web assets and the Go binaries:
   ```bash
   make
   ```
   This will produce the following binaries in the `bin/` directory:
   - `llm-monitor-proxy`: The monitoring proxy server.
   - `llm-monitor-api`: The API server that also serves the embedded Web UI.

2. **Run the Proxy**:
   ```bash
   ./bin/llm-monitor-proxy -c configs/config.yaml
   ```

3. **Run the API / Web UI**:
   ```bash
   ./bin/llm-monitor-api -c configs/config.yaml
   ```

## Configuration

The application is configured via a YAML file (default `config.yaml`). You can use environment variables within the YAML file using the `${VAR:-default}` syntax.

### Example `config.yaml`

```yaml
logging:
  format: "text"  # "json" or "text"

proxy:
  port: 8080
  upstream:
    url: "${UPSTREAM_URL:-http://localhost:11434}"
  intercepts:
    - endpoint: "/api/generate"
      interceptor: "OllamaGenerateInterceptor"
    - endpoint: "/api/chat"
      interceptor: "OllamaChatInterceptor"

api:
  port: 8081

storage:
  type: "postgres"
  postgres:
    dsn: "postgres://${DB_USER:-user}:${DB_PASSWORD:-password}@${DB_HOST:-localhost}:${DB_PORT:-5432}/${DB_NAME:-llm_monitor}?sslmode=disable"
```

### Environment Variables

| Variable       | Description                           | Default                  |
|----------------|---------------------------------------|--------------------------|
| `UPSTREAM_URL` | The URL of the LLM server to proxy to | `http://localhost:11434` |
| `DB_USER`      | PostgreSQL user                       | `user`                   |
| `DB_PASSWORD`  | PostgreSQL password                   | `password`               |
| `DB_HOST`      | PostgreSQL host                       | `localhost`              |
| `DB_PORT`      | PostgreSQL port                       | `5432`                   |
| `DB_NAME`      | PostgreSQL database name              | `llm_monitor`            |

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
