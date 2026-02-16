.PHONY: all build-web build-go clean

# Default target
all: build-web build-go

# Build the web UI
build-web:
	cd web && npm install && npm run build

# Build all Go commands
build-go:
	go build -o bin/llm-monitor-proxy cmd/proxy/main.go
	go build -o bin/llm-monitor-api cmd/api/main.go

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf web/dist
