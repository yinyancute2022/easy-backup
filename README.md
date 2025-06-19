# Easy Backup - Database Backup Tool

A simple, reliable tool for automated database backups with S3 storage and Slack notifications.

## Features

- **Multiple Database Support**: PostgreSQL, MySQL/MariaDB, MongoDB
- **Flexible Scheduling**: Cron-based backup scheduling
- **S3 Storage**: Automatic upload to S3-compatible storage
- **Slack Notifications**: Real-time backup status updates
- **Manual Triggers**: Execute backups on-demand
- **Health Monitoring**: Built-in health checks and Prometheus metrics
- **Retry Logic**: Configurable retry attempts for failed backups

## Installation

### Download Pre-built Binaries

Download the latest release from the [GitHub Releases page](https://github.com/yinyancute2022/db-backup/releases).

```bash
# Example: Download and extract for Linux amd64
wget https://github.com/yinyancute2022/db-backup/releases/latest/download/db-backup-v1.0.0-linux-amd64.tar.gz
tar -xzf db-backup-v1.0.0-linux-amd64.tar.gz
chmod +x easy-backup-v1.0.0-linux-amd64
sudo mv easy-backup-v1.0.0-linux-amd64 /usr/local/bin/easy-backup
```

### Docker Image

```bash
# Pull the latest Docker image
docker pull ghcr.io/yinyancute2022/db-backup:latest
```

## Quick Start

### 1. Create Configuration

Create a `config.yaml` file based on the example:

```yaml
global:
  slack:
    bot_token: "xoxb-your-slack-bot-token"
    channel_id: "C0123456789"
  log_level: "info"
  schedule: "0 2 * * *" # Daily at 2 AM
  retention: "30d"
  s3:
    bucket: "my-backup-bucket"
    base_path: "database-backups"
    credentials:
      access_key: "your-access-key"
      secret_key: "your-secret-key"
      region: "us-east-1"

strategies:
  - name: "postgres-prod"
    database_type: "postgres"
    database_url: "postgres://user:pass@host:5432/db"
    schedule: "0 */6 * * *" # Every 6 hours

  - name: "mysql-app"
    database_type: "mysql"
    database_url: "mysql://user:pass@host:3306/db"
```

### 2. Validate Configuration

```bash
# Validate your configuration
config-validator config.yaml
```

### 3. Run Backup Service

```bash
# Run as a service
./easy-backup -config config.yaml

# Run with Docker
docker run -v $(pwd)/config.yaml:/app/config.yaml \
  ghcr.io/yinyancute2022/db-backup:latest
```

## Manual Backup Execution

Execute backups manually without running the scheduler service:

```bash
# Execute all backup strategies once
./easy-backup -config config.yaml -manual

# Execute specific strategy
./easy-backup -config config.yaml -strategy "postgres-prod"

# Show help
./easy-backup --help
```

**Use Cases:**

- Testing configuration before scheduling
- One-time backups before maintenance
- CI/CD pipeline integration
- Troubleshooting specific strategies

## Monitoring

### Health Check

```bash
curl http://localhost:8080/health
```

### Prometheus Metrics

```bash
curl http://localhost:8080/metrics
```

## Schedule Configuration

Use cron expressions for flexible scheduling:

```yaml
schedule: "0 2 * * *"        # Daily at 2 AM
schedule: "*/15 * * * *"     # Every 15 minutes
schedule: "0 */6 * * *"      # Every 6 hours
schedule: "0 0 * * 1"        # Weekly on Monday
schedule: "0 6 1 * *"        # Monthly on 1st at 6 AM
```

## Configuration Reference

### Global Settings

| Setting            | Description             | Example                   |
| ------------------ | ----------------------- | ------------------------- |
| `slack.bot_token`  | Slack bot token         | `xoxb-...`                |
| `slack.channel_id` | Slack channel ID        | `C0123456789`             |
| `log_level`        | Logging level           | `info`, `debug`, `error`  |
| `schedule`         | Default backup schedule | `0 2 * * *`               |
| `retention`        | Backup retention period | `30d`, `7d`, `1h`         |
| `timezone`         | Timezone for schedules  | `UTC`, `America/New_York` |
| `s3.bucket`        | S3 bucket name          | `my-backup-bucket`        |
| `s3.base_path`     | S3 path prefix          | `database-backups`        |
| `s3.compression`   | Compression type        | `gzip`, `none`            |

### Strategy Settings

| Setting         | Description               | Example                             |
| --------------- | ------------------------- | ----------------------------------- |
| `name`          | Strategy identifier       | `postgres-prod`                     |
| `database_type` | Database type             | `postgres`, `mysql`, `mongodb`      |
| `database_url`  | Database connection URL   | `postgres://user:pass@host:5432/db` |
| `schedule`      | Override global schedule  | `0 */6 * * *`                       |
| `retention`     | Override global retention | `7d`                                |

### Database URL Formats

```bash
# PostgreSQL
postgres://username:password@hostname:5432/database

# MySQL/MariaDB
mysql://username:password@hostname:3306/database

# MongoDB
mongodb://username:password@hostname:27017/database
```

## Environment Variables

You can use environment variables in your configuration:

```yaml
global:
  slack:
    bot_token: "${SLACK_BOT_TOKEN}"
  s3:
    bucket: "${S3_BUCKET}"
    credentials:
      access_key: "${AWS_ACCESS_KEY_ID}"
      secret_key: "${AWS_SECRET_ACCESS_KEY}"

strategies:
  - name: "postgres-prod"
    database_url: "${DATABASE_URL}"
```

## Examples

See the `examples/` directory for:

- Complete Docker Compose setup
- Sample configuration files
- Test data generation scripts

## Troubleshooting

### Common Issues

1. **Database Connection Failed**

   - Verify database URL format
   - Check network connectivity
   - Ensure database client tools are installed

2. **S3 Upload Failed**

   - Verify AWS credentials
   - Check bucket permissions
   - Ensure bucket exists

3. **Slack Notifications Not Working**
   - Verify bot token format (starts with `xoxb-`)
   - Check channel ID (not channel name)
   - Ensure bot has permissions to post

### Debug Mode

Enable debug logging for troubleshooting:

```yaml
global:
  log_level: "debug"
```

## License

This project is licensed under the MIT License.

## Support

- [GitHub Issues](https://github.com/yinyancute2022/db-backup/issues)
- [Documentation](https://github.com/yinyancute2022/db-backup/tree/main/examples)
