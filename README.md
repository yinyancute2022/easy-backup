# Database Backup Tool

Project name easy backup

## Installation

### Download Pre-built Binaries

Download the latest release from the [GitHub Releases page](https://github.com/yinyancute2022/db-backup/releases).

Available for:

- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64, arm64)

```bash
# Example: Download and extract for Linux amd64
wget https://github.com/yinyancute2022/db-backup/releases/latest/download/db-backup-v1.0.0-linux-amd64.tar.gz
tar -xzf db-backup-v1.0.0-linux-amd64.tar.gz

# Make binaries executable
chmod +x easy-backup-v1.0.0-linux-amd64
chmod +x config-validator-v1.0.0-linux-amd64

# Optionally, move to PATH
sudo mv easy-backup-v1.0.0-linux-amd64 /usr/local/bin/easy-backup
sudo mv config-validator-v1.0.0-linux-amd64 /usr/local/bin/config-validator
```

### Docker Image

```bash
# Pull the latest Docker image
docker pull ghcr.io/yinyancute2022/db-backup:latest

# Or use a specific version
docker pull ghcr.io/yinyancute2022/db-backup:v1.0.0
```

### Build from Source

```bash
# Clone the repository
git clone https://github.com/yinyancute2022/db-backup.git
cd db-backup

# Build the binaries
make build

# Or build manually
go build -o easy-backup ./cmd/easy-backup
go build -o config-validator ./cmd/config-validator
```

## Quick Start

### 1. Validate Configuration

```bash
# Validate your configuration file
./config-validator path/to/your/config.yaml
```

### 2. Run Backup Service

```bash
# Run with configuration file
./easy-backup -config path/to/your/config.yaml

# Run with Docker
docker run -v $(pwd)/config.yaml:/app/config.yaml \
  -v $(pwd)/.env:/app/.env \
  ghcr.io/yinyancute2022/db-backup:latest
```

### 3. Monitor Health

```bash
# Check health status
curl http://localhost:8080/health

# View metrics
curl http://localhost:8080/metrics
```

## Manual Backup Execution

The tool supports manual execution of backup strategies without running the scheduler service. This is useful for testing, one-time backups, or integrating with external systems.

### Manual Trigger All Strategies

Execute all configured backup strategies once and exit:

```bash
# Execute all backup strategies manually
./easy-backup -config path/to/your/config.yaml -manual
```

This will:

- Execute all strategies defined in the configuration
- Apply retry logic and error handling
- Send Slack notifications (if configured)
- Upload successful backups to S3
- Exit after completion (no scheduler service)

### Manual Trigger Single Strategy

Execute a specific backup strategy by name:

```bash
# Execute a specific strategy manually
./easy-backup -config path/to/your/config.yaml -strategy "postgres-prod"
```

This will:

- Execute only the specified strategy
- Apply the same retry and notification logic
- Exit after completion

### Use Cases for Manual Execution

- **Testing Configuration**: Verify backup strategies work before scheduling
- **One-time Backups**: Create backups before maintenance or deployments
- **CI/CD Integration**: Trigger backups as part of deployment pipelines
- **Troubleshooting**: Debug specific backup strategies in isolation
- **External Orchestration**: Integrate with external job schedulers or cron

### Manual Execution Examples

```bash
# Test all strategies before going to production
./easy-backup -config production.yaml -manual

# Create backup before database migration
./easy-backup -config config.yaml -strategy "production-postgres"

# Verify specific strategy configuration
./easy-backup -config config.yaml -strategy "mysql-app"

# Use in CI/CD pipeline (with proper error handling)
if ./easy-backup -config config.yaml -strategy "staging-db"; then
    echo "Backup successful, proceeding with deployment"
else
    echo "Backup failed, aborting deployment"
    exit 1
fi
```

### Command Line Options

```bash
Usage of ./easy-backup:
  -config string
        Path to configuration file (default "config.yaml")
  -manual
        Execute all backup strategies manually and exit
  -strategy string
        Execute a specific backup strategy manually and exit
```

**Note**: Manual execution uses the same configuration, retry logic, notifications, and S3 upload as scheduled backups. The only difference is that it runs once and exits instead of running as a persistent service.
