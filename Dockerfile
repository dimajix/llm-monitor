# Build web stage
FROM node:20-alpine AS web-builder
WORKDIR /web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy built web assets
COPY --from=web-builder /web/dist ./web/dist

# Build the application
RUN CGO_ENABLED=0 GOOS=linux \
    go build -o llm-monitor-proxy cmd/proxy/main.go \
    && go build -o llm-monitor-api cmd/api/main.go

# Final stage
FROM alpine:latest

# Create a non-privileged user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/llm-monitor-proxy .
COPY --from=builder /app/llm-monitor-api .
COPY --from=builder /app/configs/config.yaml ./config/config.yaml

# Set ownership to the non-privileged user
RUN chown -R appuser:appgroup /app

# Use the non-privileged user
USER appuser

# Expose the port the app runs on
EXPOSE 8080

# Command to run the application
CMD ["./llm-monitor-proxy", "-c", "/app/config/config.yaml"]
