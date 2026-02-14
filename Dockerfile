# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o llm-monitor cmd/main.go

# Final stage
FROM alpine:latest

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/llm-monitor .
COPY configs/config.yaml /app/config/config.yaml

# Expose the port the app runs on
EXPOSE 8080

# Command to run the application
CMD ["./llm-monitor", "-c", "/app/config/config.yaml"]
