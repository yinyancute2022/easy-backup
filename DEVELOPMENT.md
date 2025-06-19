# Development Guide

This guide covers development, building, testing, and deployment of the Easy Backup tool.

## Prerequisites

- Go 1.24 or later
- Make
- Docker (for containerized builds)
- Database client tools (for testing):
  - PostgreSQL client (`pg_dump`, `pg_restore`)
  - MySQL/MariaDB client (`mariadb-dump`)
  - MongoDB client (`mongodump`)

## Quick Development Setup

```bash
# Clone and setup
git clone <repository-url>
cd db-backup
make deps

# Install development tools
make install-tools

# Create configuration
cp config.example.yaml config.yaml
# Edit config.yaml with your values

# Run integration test
make integration-test-quick
```

## Project Requirements & Architecture

### Technology Stack

- **Language**: Go 1.24+
- **Container**: Docker (Alpine Linux base)
- **CI/CD**: GitHub Actions
- **Registry**: GitHub Container Registry
- **Monitoring**: Prometheus metrics, health checks
- **Notifications**: Slack API
- **Storage**: S3-compatible storage

### Core Features

#### Configuration Management

- **YAML-based configuration** with environment variable substitution
- **Two-tier structure**: Global defaults + Strategy overrides
- **Validation**: Built-in configuration validator
- **Environment integration**: Support for `.env` files

#### Database Support

- **PostgreSQL**: Using `pg_dump` and `pg_restore`
- **MySQL/MariaDB**: Using `mariadb-dump`
- **MongoDB**: Using `mongodump` with tar.gz compression
- **Auto-detection**: Database type detection from connection URLs
- **Multiple connections**: Support for multiple database instances

#### Backup Management

- **Flexible scheduling**: Cron expression support
- **Compression**: Optional gzip compression (configurable)
- **S3 upload**: Automatic upload with configurable paths
- **Retention**: Time-based backup retention
- **Retry logic**: Configurable retry attempts with backoff
- **Timeout handling**: Configurable timeouts for backup and upload

#### Monitoring & Observability

- **Health endpoints**: JSON-based health status API
- **Prometheus metrics**: Custom metrics for backup operations
- **Structured logging**: Configurable log levels with JSON output
- **Command output capture**: Full shell command output logging
- **Status tracking**: Real-time backup status monitoring

#### Notifications

- **Slack integration**: Threaded notifications with progress updates
- **Status updates**: Start, progress, success/failure notifications
- **Error details**: Detailed error information in notifications
- **Channel management**: Per-strategy notification channels
- **Bot API**: Uses Slack Bot API with bot tokens (xoxb-\*)

#### Manual Execution

- **On-demand backups**: Execute all or specific strategies manually
- **CI/CD integration**: Support for automated pipeline integration
- **Testing support**: Validate configurations before scheduling
- **Troubleshooting**: Execute strategies in isolation

## Development Workflow

### Building

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Build Docker image
make docker-build

# Clean build artifacts
make clean
```

### Testing

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run integration tests (recommended)
make integration-test-quick    # Fast integration test
make integration-test         # Full integration test
```

### Integration Testing

The integration tests provide comprehensive validation:

**Quick Integration Test** (`make integration-test-quick`):

- ✅ Dependencies installation
- ✅ Code formatting and vetting
- ✅ Unit test execution
- ✅ Binary compilation
- ✅ Binary execution validation
- ✅ Configuration validation

**Full Integration Test** (`make integration-test`):

- ✅ All quick test steps
- ✅ Cross-platform builds
- ✅ Docker image building
- ✅ Security vulnerability checks

### Code Quality

```bash
# Format code
make fmt

# Vet code
make vet

# Security check
make security

# Generate mocks
make mocks
```

### Development Server

```bash
# Run with hot reload (requires air)
make dev

# Run normally
make run

# Validate configuration
make validate-config
```

## Project Structure

```
.
├── cmd/                    # Command line applications
│   ├── easy-backup/        # Main backup application
│   └── config-validator/   # Configuration validator
├── internal/               # Internal packages (not for external use)
│   ├── backup/            # Core backup execution logic
│   ├── config/            # Configuration parsing and validation
│   ├── logger/            # Structured logging setup
│   ├── monitoring/        # Health checks and Prometheus metrics
│   ├── notification/      # Slack notification service
│   ├── scheduler/         # Cron-based backup scheduling
│   └── storage/           # S3 storage operations
├── examples/              # Example configurations and Docker setup
├── dist/                  # Build output (gitignored)
├── .github/workflows/     # GitHub Actions CI/CD
├── config.example.yaml    # Example configuration file
├── Dockerfile            # Multi-stage container build
├── Makefile             # Build automation and development tasks
└── go.mod               # Go module definition
```

## Configuration System

### Configuration Structure

```yaml
global: # Default settings for all strategies
  slack: # Slack notification settings
    bot_token: string # Slack bot token (xoxb-*)
    channel_id: string # Default Slack channel ID
  log_level: string # info, debug, warn, error
  schedule: string # Default cron schedule
  retention: string # Default retention period (30d, 7d, etc.)
  timezone: string # Timezone for scheduling (UTC, etc.)
  temp_dir: string # Temporary directory for backups
  max_parallel_strategies: int # Max concurrent backup executions
  retry:
    max_attempts: int # Number of retry attempts
  timeout:
    backup: string # Backup operation timeout
    upload: string # S3 upload timeout
  s3: # S3 storage configuration
    bucket: string # S3 bucket name
    base_path: string # Base path for backups
    compression: string # gzip or none
    endpoint: string # Custom S3 endpoint (optional)
    credentials:
      access_key: string # AWS access key
      secret_key: string # AWS secret key
      region: string # AWS region
  monitoring: # Monitoring configuration
    metrics:
      enabled: bool # Enable Prometheus metrics
      port: int # Metrics server port
      path: string # Metrics endpoint path
    health_check:
      port: int # Health check server port
      path: string # Health check endpoint path

strategies: # Array of backup strategies
  - name: string # Unique strategy identifier
    database_type: string # postgres, mysql, mongodb
    database_url: string # Database connection URL
    schedule: string # Override global schedule (optional)
    retention: string # Override global retention (optional)
    slack: # Override global Slack settings (optional)
      channel_id: string # Strategy-specific channel
```

### Environment Variable Substitution

The configuration system supports environment variable substitution:

```yaml
global:
  slack:
    bot_token: "${SLACK_BOT_TOKEN}"
  s3:
    bucket: "${S3_BUCKET}"
    credentials:
      access_key: "${AWS_ACCESS_KEY_ID}"
      secret_key: "${AWS_SECRET_ACCESS_KEY}"
      region: "${AWS_REGION}"

strategies:
  - name: "postgres-prod"
    database_url: "${POSTGRES_DATABASE_URL}"
```

## Docker Development

### Building and Running

```bash
# Build Docker image
make docker-build

# Run with configuration
docker run -v $(pwd)/config.yaml:/app/config.yaml \
  ghcr.io/yinyancute2022/db-backup:latest

# Run with environment variables
docker run -e SLACK_BOT_TOKEN=xoxb-... \
  -e S3_BUCKET=my-bucket \
  ghcr.io/yinyancute2022/db-backup:latest
```

### Example Environment

A complete example environment with sample data:

```bash
# Start example environment
cd examples
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs easy-backup

# Test manual backup
docker-compose exec easy-backup ./easy-backup -manual

# Cleanup
docker-compose down
```

## API Reference

### Health Check Endpoint

**GET** `/health`

Returns JSON health status:

```json
{
  "status": "healthy",
  "timestamp": "2025-06-19T15:30:00Z",
  "version": "v1.0.0",
  "strategies": {
    "total": 3,
    "last_backup_status": {
      "postgres-prod": {
        "status": "success",
        "last_run": "2025-06-19T15:00:00Z",
        "next_run": "2025-06-19T21:00:00Z"
      }
    }
  },
  "s3_connectivity": "healthy",
  "slack_connectivity": "healthy"
}
```

### Metrics Endpoint

**GET** `/metrics`

Returns Prometheus metrics:

```
# HELP backup_duration_seconds Duration of backup operations
# TYPE backup_duration_seconds histogram
backup_duration_seconds_bucket{strategy="postgres-prod",le="30"} 1
backup_duration_seconds_count{strategy="postgres-prod"} 1
backup_duration_seconds_sum{strategy="postgres-prod"} 15.2

# HELP backup_size_bytes Size of backup files
# TYPE backup_size_bytes gauge
backup_size_bytes{strategy="postgres-prod"} 1048576

# HELP backup_success_total Total successful backups
# TYPE backup_success_total counter
backup_success_total{strategy="postgres-prod"} 24

# HELP backup_failures_total Total failed backups
# TYPE backup_failures_total counter
backup_failures_total{strategy="postgres-prod"} 1
```

## Testing Strategy

### Unit Tests

```bash
# Run specific package tests
go test ./internal/config
go test -v ./internal/backup
go test -cover ./internal/notification
```

### Integration Tests

```bash
# Quick integration test (development)
make integration-test-quick

# Full integration test (CI/release)
make integration-test
```

### Manual Testing

```bash
# Test configuration validation
./config-validator examples/backup-config.yaml

# Test manual backup execution
./easy-backup -config examples/backup-config.yaml -manual

# Test specific strategy
./easy-backup -config examples/backup-config.yaml -strategy "postgres-prod"

# Test health endpoint
curl http://localhost:8080/health | jq '.'

# Test metrics endpoint
curl http://localhost:8080/metrics
```

## Deployment

### GitHub Actions Workflows

**Build Workflow** (`.github/workflows/build.yml`):

- Triggers on push to `main`/`develop` and PRs
- Runs tests and builds Docker images
- Pushes Docker images only for `main` branch

**Release Workflow** (`.github/workflows/release.yml`):

- Triggers on release creation
- Builds multi-platform binaries
- Creates release archives
- Builds and pushes tagged Docker images

### Release Process

```bash
# Create and tag release
git tag v1.1.0
git push origin v1.1.0

# Create GitHub release (triggers release workflow)
gh release create v1.1.0 --title "Release v1.1.0" --notes "Release notes..."
```

## Makefile Reference

| Target                   | Description                          |
| ------------------------ | ------------------------------------ |
| `all`                    | Full build: clean, deps, test, build |
| `deps`                   | Install Go dependencies              |
| `fmt`                    | Format Go code                       |
| `vet`                    | Vet Go code for issues               |
| `test`                   | Run unit tests                       |
| `test-coverage`          | Run tests with coverage report       |
| `build`                  | Build binaries for current platform  |
| `build-all`              | Build binaries for all platforms     |
| `docker-build`           | Build Docker image                   |
| `run`                    | Run application locally              |
| `dev`                    | Run with hot reload (requires air)   |
| `clean`                  | Clean all build artifacts            |
| `integration-test-quick` | Quick integration test               |
| `integration-test`       | Full integration test                |
| `security`               | Run security vulnerability scan      |
| `mocks`                  | Generate test mocks                  |
| `install-tools`          | Install development tools            |
| `help`                   | Show all available targets           |

## Troubleshooting

### Common Development Issues

**Build Failures:**

- Ensure Go 1.24+ is installed: `go version`
- Update dependencies: `make deps`
- Install development tools: `make install-tools`

**Test Failures:**

- Check test output for specific failures
- Run tests with verbose output: `go test -v ./...`
- Run integration tests: `make integration-test-quick`

**Docker Issues:**

- Ensure Docker daemon is running
- Check Dockerfile syntax
- Verify base image availability: `docker pull alpine:latest`

**Runtime Issues:**

- Validate configuration: `./config-validator config.yaml`
- Check application logs in debug mode
- Verify external service connectivity (databases, S3, Slack)
- Check health endpoint: `curl http://localhost:8080/health`

### Debugging

**Enable Debug Logging:**

```yaml
global:
  log_level: "debug"
```

**View Structured Logs:**

```bash
./easy-backup -config config.yaml 2>&1 | jq '.'
```

**Check Service Health:**

```bash
curl -s http://localhost:8080/health | jq '.'
curl -s http://localhost:8080/metrics
```

## Contributing

### Development Process

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/new-feature`
3. Make changes following coding standards
4. Run tests: `make integration-test-quick`
5. Commit changes: `git commit -m "Add new feature"`
6. Push branch: `git push origin feature/new-feature`
7. Create pull request

### Coding Standards

- **Formatting**: Use `go fmt` for consistent formatting
- **Naming**: Follow Go naming conventions
- **Documentation**: Add godoc comments for public functions
- **Testing**: Include unit tests for new functionality
- **Error handling**: Use structured error handling with context
- **Logging**: Use structured logging with appropriate levels

### Adding New Features

**New Database Type:**

1. Extend `internal/backup/backup.go` with new database logic
2. Add database-specific dump/restore commands
3. Update configuration validation in `internal/config/config.go`
4. Add tests for new database type
5. Update documentation

**New Notification Channel:**

1. Create new service in `internal/notification/`
2. Implement notification interface
3. Update configuration structure
4. Integrate with scheduler service
5. Add tests and documentation

**New Metrics:**

1. Define metrics in `internal/monitoring/monitoring.go`
2. Register metrics in `NewMonitoringService`
3. Record metrics in appropriate service methods
4. Add metric documentation
   make fmt

# Vet code

make vet

# Security check

make security

````

### Development Server

```bash
# Run with hot reload (requires air)
make dev

# Or run normally
make run
````

### Configuration Validation

```bash
# Validate configuration file
make validate-config
```

## Makefile Targets

| Target            | Description                              |
| ----------------- | ---------------------------------------- |
| `all`             | Run clean, deps, test, and build         |
| `deps`            | Install dependencies                     |
| `fmt`             | Format Go code                           |
| `vet`             | Vet Go code                              |
| `test`            | Run tests                                |
| `test-coverage`   | Run tests with coverage report           |
| `test-short`      | Run short tests                          |
| `build`           | Build for current platform               |
| `build-all`       | Build for all platforms                  |
| `docker-build`    | Build Docker image                       |
| `docker-push`     | Build and push Docker image              |
| `run`             | Run the application locally              |
| `validate-config` | Run config validator                     |
| `dev`             | Start development server with hot reload |
| `clean`           | Clean build artifacts                    |
| `clean-dist`      | Clean dist directory                     |
| `clean-test`      | Clean test artifacts                     |
| `update-deps`     | Update dependencies                      |
| `security`        | Run security checks                      |
| `mocks`           | Generate mocks                           |
| `release`         | Prepare release build                    |
| `install-tools`   | Install development tools                |
| `help`            | Show help message                        |

## Project Structure

```
.
├── cmd/                    # Command line applications
│   ├── easy-backup/        # Main backup application
│   └── config-validator/   # Configuration validator tool
├── internal/               # Internal packages
│   ├── backup/            # Backup execution logic
│   ├── config/            # Configuration management
│   ├── logger/            # Logging setup
│   ├── monitoring/        # Health checks and metrics
│   ├── notification/      # Slack notifications
│   ├── scheduler/         # Backup scheduling
│   └── storage/           # S3 storage operations
├── dist/                  # Build output (gitignored)
├── config.example.yaml    # Example configuration
├── Dockerfile             # Container definition
├── Makefile              # Build automation
├── .air.toml             # Hot reload configuration
└── go.mod                # Go module definition
```

## Configuration

The application uses YAML configuration with environment variable substitution. See `config.example.yaml` for the complete structure.

### Key Configuration Sections

- **Global**: Default settings for all backup strategies
- **Strategies**: Individual database backup configurations

### Environment Variables

Set these environment variables or include them in your configuration:

```bash
export SLACK_BOT_TOKEN="xoxb-your-slack-bot-token"
export SLACK_CHANNEL_ID="C1234567890"
export S3_BUCKET="your-backup-bucket"
export AWS_ACCESS_KEY_ID="your-aws-access-key"
export AWS_SECRET_ACCESS_KEY="your-aws-secret-key"
export AWS_REGION="us-east-1"
export DATABASE_URL="postgres://user:pass@host:port/db"
```

## Docker Development

### Building Docker Image

```bash
make docker-build
```

### Running in Docker

```bash
docker run -v $(pwd)/config.yaml:/app/config.yaml ghcr.io/your-org/easy-backup:latest
```

### Docker Compose Example

A complete example environment is provided with PostgreSQL, MinIO (S3), and sample data:

```bash
# Start example environment (includes PostgreSQL, MinIO, data generator, and backup service)
make example-up

# View logs
make example-logs

# Check status
make example-status

# Run full test
make example-test

# Cleanup
make example-down
```

The example runs backups every 1 minute with 10-minute retention. See `examples/README.md` for detailed usage.

## Monitoring and Health Checks

### Health Check Endpoint

```bash
curl http://localhost:8080/health
```

### Metrics Endpoint (Prometheus)

```bash
curl http://localhost:8080/metrics
```

## Debugging

### Enable Debug Logging

Set `log_level: "debug"` in your configuration file.

### View Application Logs

```bash
make run 2>&1 | jq '.'
```

### Check Health Status

```bash
curl -s http://localhost:8080/health | jq '.'
```

## Common Development Tasks

### Adding a New Database Type

1. Extend the backup service in `internal/backup/backup.go`
2. Add database-specific logic for dump/restore commands
3. Update configuration validation
4. Add tests for the new functionality

### Adding New Metrics

1. Define new Prometheus metrics in `internal/monitoring/monitoring.go`
2. Register metrics in `NewMonitoringService`
3. Record metrics in appropriate service methods

### Adding New Notification Channels

1. Create new service in `internal/notification/`
2. Implement notification interface
3. Update configuration structure
4. Integrate with scheduler service

## Testing Strategy

### Unit Tests

Place test files alongside source files with `_test.go` suffix.

```bash
# Run tests for specific package
go test ./internal/config

# Run with verbose output
go test -v ./internal/config

# Run with coverage
go test -cover ./internal/config
```

### Integration Tests

Use build tags for integration tests:

```go
//go:build integration
// +build integration

package integration_test
```

Run with:

```bash
go test -tags=integration ./...
```

## Release Process

### Preparing a Release

```bash
# Run full release build
make release

# Tag the release
git tag v1.0.0
git push origin v1.0.0
```

### GitHub Actions

The project includes GitHub Actions for:

- Running tests on pull requests
- Building and pushing Docker images
- Creating release binaries

## Troubleshooting

### Build Issues

1. Ensure Go 1.24+ is installed
2. Run `make deps` to update dependencies
3. Check for missing development tools with `make install-tools`

### Runtime Issues

1. Verify configuration with `make validate-config`
2. Check application logs for errors
3. Verify external service connectivity (S3, Slack)
4. Check health endpoint for service status

### Docker Issues

1. Ensure Docker daemon is running
2. Check Dockerfile syntax
3. Verify base image availability
4. Check container logs: `docker logs <container-id>`

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes following the coding standards
4. Run tests: `make test`
5. Submit a pull request

### Coding Standards

- Use `go fmt` for formatting
- Follow Go naming conventions
- Add tests for new functionality
- Update documentation as needed
