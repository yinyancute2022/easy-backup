# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binaries for Linux
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev') -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -a -installsuffix cgo \
    -o easy-backup \
    ./cmd/easy-backup

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev') -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -a -installsuffix cgo \
    -o config-validator \
    ./cmd/config-validator

# Final stage
FROM alpine:3.22

# Install required packages
RUN apk update && apk add --no-cache \
    postgresql17-client \
    mariadb-client \
    mariadb-connector-c-dev \
    mongodb-tools \
    ca-certificates \
    tzdata \
    wget

# Create app directory
WORKDIR /app

# Copy the binaries from builder stage
COPY --from=builder /app/easy-backup /app/easy-backup
COPY --from=builder /app/config-validator /app/config-validator

# Create temp directory
RUN mkdir -p /tmp/db-backup && chmod 755 /tmp/db-backup

# Create non-root user
RUN adduser -D -s /bin/sh appuser && chown -R appuser:appuser /app /tmp/db-backup
USER appuser

# Expose port for health checks and metrics
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["/app/easy-backup"]
