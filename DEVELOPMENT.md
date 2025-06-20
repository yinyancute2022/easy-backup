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

#### Startup Execution

- **Execute on startup**: Configurable immediate execution of all strategies when service starts
- **Deployment integration**: Ensures backups are created immediately after deployment
- **Recovery scenarios**: Provides fresh backups after service restarts
- **Development workflow**: Quick backup verification during development

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

# Or run normally
make run
```

### Configuration Validation

```bash
# Validate configuration file
make validate-config
```

### Timezone Testing

The application supports IANA timezone identifiers for scheduling backups in different timezones. Test timezone functionality:

```bash
# Run timezone-specific tests
go test ./internal/scheduler -v -run TestTimezone

# Test timezone configuration with config validator
./dist/config-validator -config examples/timezone-config.yaml

# Validate specific timezone configurations
./dist/config-validator -config config.yaml | grep "Timezone:"
```

**Common Timezone Examples:**
- `UTC` - Coordinated Universal Time
- `America/New_York` - Eastern Time (US)
- `America/Los_Angeles` - Pacific Time (US)  
- `Europe/London` - Greenwich Mean Time
- `Asia/Tokyo` - Japan Standard Time

**Timezone Features:**
- Automatic DST handling
- Invalid timezone fallback to UTC
- Per-strategy timezone inheritance from global settings
- Cron schedule evaluation in specified timezone
```

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
