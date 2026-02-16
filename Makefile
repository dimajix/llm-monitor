.PHONY: all build-web build-go clean

# Default target
all: build-web build-go

# Build the web UI
build-web:
	cd web && npm install && npm run build

# Build all Go commands
# We use 'go build ./cmd/...' to build all binaries in the cmd directory
build-go:
	go build -o bin/ ./cmd/...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf web/dist
