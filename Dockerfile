# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')" \
    -o /auth-proxy \
    ./cmd/server

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

# Copy binary from builder
COPY --from=builder /auth-proxy /usr/local/bin/auth-proxy

# Set ownership
RUN chown appuser:appgroup /usr/local/bin/auth-proxy

# Use non-root user
USER appuser

# Expose ports (gRPC and metrics)
EXPOSE 50051 9090

# Health check using grpc_health_probe
# Install grpc_health_probe for health checks
# For production, consider adding grpc_health_probe binary
# HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
#     CMD grpc_health_probe -addr=:50051 || exit 1

# Run the binary
ENTRYPOINT ["/usr/local/bin/auth-proxy"]
