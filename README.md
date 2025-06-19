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

## Project Requirements

### Technology Stack

- Primary development language: Go
- Container: Docker image based on Alpine Linux
- CI/CD: GitHub Actions
- Registry: GitHub Container Registry

### Core Features

#### Configuration

- Use YAML configuration file with two main sections:
  - **Global**: Default configurations for all strategies
  - **Strategies**: Array of backup configurations for specific databases - Global section includes:
  - Slack bot token and default channel ID
  - Default log level
  - Default backup schedule
  - Default retention period
  - Retry and timeout configurations
  - Timezone settings
  - Parallel execution limits
  - S3 configuration (bucket, base path, credentials)
  - Backup compression settings
- Each strategy contains:
  - Strategy name as key
  - Database URL and connection details
  - Override configurations for global defaults

#### Database Support

- **PostgreSQL** backup using `pg_dump` and `pg_restore`
- **MySQL/MariaDB** backup using `mariadb-dump`
- **MongoDB** backup using `mongodump` with tar.gz compression
- Support multiple database URLs
- Support database connections through SSH tunnels or proxies
- Automatic database type detection based on configuration

#### Backup Management

- **Flexible schedule configuration**:
  - **Cron expressions**: `"0 2 * * *"` (daily at 2 AM), `"*/15 * * * *"` (every 15 minutes)
- Upload backups to S3 storage with gzip compression by default
- Time zone handling for schedules

#### Monitoring & Logging

- Expose health check endpoints that return JSON status
- Provide Prometheus metrics on configurable endpoint
- Support configurable log levels
- Collect and log all shell command outputs
- Essential logs for debugging and process tracking
- Configurable metrics and health check ports and paths

#### Notifications

- Send Slack messages in threaded format:
  - Initial message when backup starts
  - Progress updates in the same thread
  - Final success/failure report in the same thread
- Include backup size, duration, and status in notifications
- Support different notification channels based on backup status
- Update initial message to indicate errors on failure

**Note**: The Slack integration uses the Slack Bot API with bot tokens (xoxb-\*) and requires channel IDs instead of channel names for better reliability and security.

#### Deployment

- Install database clients (PostgreSQL, MariaDB, MongoDB) in Docker image
- Keep the project simple and maintainable

### Configuration Example

```yaml
global:
  slack:
    bot_token: "xoxb-your-slack-bot-token"
    channel_id: "C0123456789" # Slack channel ID
  log_level: "info"
  schedule: "0 2 * * *" # Daily at 2 AM
  retention: "30d"
  timezone: "UTC"
  temp_dir: "/tmp/db-backup"
  max_parallel_strategies: 2
  retry:
    max_attempts: 3
  timeout:
    backup: "30m"
    upload: "10m"
  s3:
    bucket: "my-backup-bucket"
    base_path: "database-backups"
    compression: "gzip"
    credentials:
      access_key: "your-access-key"
      secret_key: "your-secret-key"
      region: "us-east-1"
  monitoring:
    metrics:
      enabled: true
      port: 8080
      path: "/metrics"
    health_check:
      port: 8080
      path: "/health"

strategies:
  - name: "production-postgres"
    database_type: "postgres" # Required: postgres, mysql, mariadb, or mongodb
    database_url: "postgres://user:pass@prod-db:5432/myapp"
    schedule: "0 */6 * * *" # Override global schedule - every 6 hours
    slack:
      channel_id: "C9876543210" # Override global channel ID

  - name: "mysql-app"
    database_type: "mysql"
    database_url: "mysql://user:pass@mysql-db:3306/appdb"
    # Uses global defaults for schedule and slack channel

  - name: "mongodb-logs"
    database_type: "mongodb"
    database_url: "mongodb://user:pass@mongodb:27017/logs"
    schedule: "0 */12 * * *" # Every 12 hours
    retention: "7d"
```

### Schedule Configuration

The tool uses cron expression format for scheduling backups:

#### Cron Expression Format

```yaml
schedule: "0 2 * * *"      # Daily at 2 AM
schedule: "*/15 * * * *"   # Every 15 minutes
schedule: "0 */6 * * *"    # Every 6 hours
schedule: "0 3,9,15,21 * * *"  # Every 6 hours starting at 3 AM
schedule: "0 0 * * 1"      # Weekly on Monday at midnight
schedule: "0 8-17 * * 1-5" # Every hour from 8 AM to 5 PM, Monday to Friday
```

**Cron format:** `minute hour day_of_month month day_of_week`

- **minute**: 0-59
- **hour**: 0-23 (24-hour format)
- **day_of_month**: 1-31
- **month**: 1-12
- **day_of_week**: 0-6 (Sunday=0)

**Common Examples:**

- `"*/5 * * * *"` - Every 5 minutes
- `"0 */2 * * *"` - Every 2 hours
- `"0 0 * * *"` - Daily at midnight
- `"0 0 * * 0"` - Weekly on Sunday at midnight
- `"0 6 1 * *"` - Monthly on the 1st at 6 AM

### Health Check Response

The health check endpoint returns JSON status:

```json
{
  "status": "healthy",
  "timestamp": "2025-06-18T10:30:00Z",
  "version": "1.0.0",
  "strategies": {
    "total": 3,
    "last_backup_status": {
      "production-postgres": {
        "status": "success",
        "last_run": "2025-06-18T06:00:00Z",
        "next_run": "2025-06-18T12:00:00Z"
      },
      "mysql-app": {
        "status": "success",
        "last_run": "2025-06-18T00:00:00Z",
        "next_run": "2025-06-19T00:00:00Z"
      },
      "mongodb-logs": {
        "status": "failed",
        "last_run": "2025-06-18T00:00:00Z",
        "next_run": "2025-06-18T12:00:00Z",
        "error": "connection timeout"
      }
    }
  },
  "s3_connectivity": "ok",
  "slack_connectivity": "ok"
}
```

## Environment Configuration

For security reasons, sensitive configuration like Slack tokens should be stored in environment variables rather than in the configuration file.

### Required Environment Variables

Create a `.env` file in the examples directory:

```bash
# Copy the example environment file from examples directory
cp examples/.env.example examples/.env
```

Edit `.env` with your actual credentials:

```bash
# Slack Configuration (required for notifications)
SLACK_BOT_TOKEN=xoxb-your-actual-bot-token-here
SLACK_CHANNEL_ID=C0123456789ABCDEF

# Database Configuration
DATABASE_URL=postgres://user:pass@host:port/database
POSTGRES_DATABASE_URL=postgres://user:pass@host:port/database
MYSQL_DATABASE_URL=mysql://user:pass@host:port/database
MONGODB_DATABASE_URL=mongodb://user:pass@host:port/database

# S3 Configuration
AWS_ACCESS_KEY_ID=your-access-key
AWS_SECRET_ACCESS_KEY=your-secret-key
S3_BUCKET=your-backup-bucket
```

### Getting Slack Credentials

1. **Bot Token**: Go to https://api.slack.com/apps → Your App → OAuth & Permissions → Bot User OAuth Token
2. **Channel ID**: In Slack, right-click on your channel → View channel details → Copy channel ID

#### Required OAuth Scopes

For basic functionality, your Slack bot needs these OAuth scopes:

- `chat:write` - Send messages to channels
- `chat:write.public` - Send messages to public channels without joining

Optional scopes for enhanced health checks:

- `channels:read` - Read public channel information (for health check validation)
- `groups:read` - Read private channel information (if using private channels)

**Note**: The application will work with basic scopes, but may show "limited" status in health checks if advanced scopes are missing.

### Database Connection String Formats

#### PostgreSQL

```
postgres://username:password@hostname:port/database
postgresql://username:password@hostname:port/database
```

#### MySQL/MariaDB

```
mysql://username:password@hostname:port/database
```

#### MongoDB

```
mongodb://username:password@hostname:port/database
mongodb+srv://username:password@hostname/database  # MongoDB Atlas
```

**Note**: For special characters in passwords, use URL encoding (e.g., `%40` for `@`, `%23` for `#`).

## Example Environment

The `examples/` directory contains a complete Docker Compose environment demonstrating:

- **Multi-Database Support**: PostgreSQL, MySQL, and MongoDB running simultaneously
- **Data Generation**: Realistic sample data continuously generated for all databases
- **Different Backup Schedules**: Each database backed up at different intervals
- **S3 Storage**: MinIO provides S3-compatible storage for backups
- **Monitoring**: Health checks and metrics endpoints for monitoring

To run the example:

```bash
cd examples
cp .env.example .env
# Edit .env with your Slack credentials
docker compose up -d
```

See `examples/README.md` for detailed instructions.
