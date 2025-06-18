# Database Backup Tool

Project name easy backup

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

- PostgreSQL backup using `pg_dump` and `pg_restore`
- Support multiple database URLs
- Support database connections through SSH tunnels or proxies

#### Backup Management

- Simple schedule configuration: `1h`, `1d`, `1w` for period and retention
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

- Install PostgreSQL client in Docker image
- Keep the project simple and maintainable

### Configuration Example

```yaml
global:
  slack:
    bot_token: "xoxb-your-slack-bot-token"
    channel_id: "C0123456789" # Slack channel ID
  log_level: "info"
  schedule: "1d"
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
  - name: "production-db"
    database_url: "postgres://user:pass@prod-db:5432/myapp"
    schedule: "6h" # Override global schedule
    slack:
      channel_id: "C9876543210" # Override global channel ID

  - name: "staging-db"
    database_url: "postgres://user:pass@staging-db:5432/myapp"
    # Uses global defaults for schedule and slack channel

  - name: "analytics-db"
    database_url: "postgres://user:pass@analytics-db:5432/analytics"
    schedule: "12h"
    retention: "7d"
```

### Health Check Response

The health check endpoint returns JSON status:

```json
{
  "status": "healthy",
  "timestamp": "2025-06-17T10:30:00Z",
  "version": "1.0.0",
  "strategies": {
    "total": 3,
    "last_backup_status": {
      "production-db": {
        "status": "success",
        "last_run": "2025-06-17T06:00:00Z",
        "next_run": "2025-06-17T12:00:00Z"
      },
      "staging-db": {
        "status": "success",
        "last_run": "2025-06-17T00:00:00Z",
        "next_run": "2025-06-18T00:00:00Z"
      },
      "analytics-db": {
        "status": "failed",
        "last_run": "2025-06-17T00:00:00Z",
        "next_run": "2025-06-17T12:00:00Z",
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

# S3 Configuration
AWS_ACCESS_KEY_ID=your-access-key
AWS_SECRET_ACCESS_KEY=your-secret-key
S3_BUCKET=your-backup-bucket
```

### Getting Slack Credentials

1. **Bot Token**: Go to https://api.slack.com/apps → Your App → OAuth & Permissions → Bot User OAuth Token
2. **Channel ID**: In Slack, right-click on your channel → View channel details → Copy channel ID

## Future Enhancements

- Add MySQL backup support
- Add MongoDB backup support
