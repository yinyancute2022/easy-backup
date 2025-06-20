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
  execute_on_startup: false # Execute all strategies immediately on boot
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

## Execute on Startup

Configure the service to execute all backup strategies immediately when it starts, before the scheduled intervals:

```yaml
global:
  execute_on_startup: true # Execute all strategies immediately on boot
```

**Use Cases:**

- **Initial Backup**: Ensure you have a backup immediately after deployment
- **Recovery Scenarios**: Get fresh backups after service restarts
- **Development/Testing**: Quickly verify backup functionality
- **Scheduled Maintenance**: Combine with scheduled restarts for regular backups

**Behavior:**

- Executes all configured strategies sequentially on service startup
- Uses the same retry logic and error handling as scheduled backups
- Sends Slack notifications and uploads to S3 as configured
- Does not interfere with regular scheduled backups
- Service continues running normally after startup execution

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

## Timezone Configuration

The `timezone` setting controls when scheduled backups execute. All cron schedules are interpreted in the specified timezone.

### Supported Timezone Formats

Easy Backup supports standard IANA timezone identifiers:

```yaml
global:
  timezone: "UTC"                    # Coordinated Universal Time (default)
  timezone: "America/New_York"       # Eastern Time (US & Canada)
  timezone: "America/Los_Angeles"    # Pacific Time (US & Canada)
  timezone: "Europe/London"          # Greenwich Mean Time / British Summer Time
  timezone: "Europe/Paris"           # Central European Time
  timezone: "Asia/Tokyo"             # Japan Standard Time
  timezone: "Asia/Shanghai"          # China Standard Time
  timezone: "Australia/Sydney"       # Australian Eastern Time
```

### How Timezone Affects Scheduling

When you set a timezone, all cron expressions are evaluated in that timezone:

```yaml
global:
  timezone: "America/New_York"
  schedule: "0 2 * * *" # 2 AM Eastern Time daily

strategies:
  - name: "postgres-prod"
    schedule: "0 14 * * *" # 2 PM Eastern Time daily
```

### Timezone Examples

#### Business Hours Backup (9 AM local time)

```yaml
global:
  timezone: "America/New_York" # Eastern timezone
  schedule: "0 9 * * *" # 9 AM ET daily
```

#### Off-Hours Backup (2 AM local time)

```yaml
global:
  timezone: "Europe/London" # UK timezone
  schedule: "0 2 * * *" # 2 AM GMT/BST daily
```

#### Multi-Region Setup

```yaml
global:
  timezone: "UTC" # Global coordination

strategies:
  - name: "us-east-db"
    schedule: "0 7 * * *" # 2 AM ET (UTC-5) or 3 AM EDT (UTC-4)
  - name: "eu-db"
    schedule: "0 1 * * *" # 1 AM GMT (UTC+0) or 2 AM BST (UTC+1)
  - name: "asia-db"
    schedule: "0 18 * * *" # 3 AM JST (UTC+9)
```

### Daylight Saving Time

Easy Backup automatically handles daylight saving time transitions:

- **Spring forward**: Skipped time slots are handled gracefully
- **Fall back**: Duplicate time slots execute only once
- **Timezone changes**: Automatic adjustment for DST boundaries

### Timezone Validation

Invalid timezones fall back to UTC with a warning:

```yaml
global:
  timezone: "Invalid/Timezone" # Falls back to UTC
```

Check logs for timezone loading errors:

```bash
# Look for timezone warnings in logs
docker logs easy-backup | grep -i timezone
```

### Common Timezone Identifiers

| Region          | Timezone Identifier   | Description                   |
| --------------- | --------------------- | ----------------------------- |
| **UTC**         | `UTC`                 | Coordinated Universal Time    |
| **US East**     | `America/New_York`    | Eastern Time (EDT/EST)        |
| **US Central**  | `America/Chicago`     | Central Time (CDT/CST)        |
| **US Mountain** | `America/Denver`      | Mountain Time (MDT/MST)       |
| **US Pacific**  | `America/Los_Angeles` | Pacific Time (PDT/PST)        |
| **UK**          | `Europe/London`       | Greenwich Mean Time (GMT/BST) |
| **Germany**     | `Europe/Berlin`       | Central European Time         |
| **France**      | `Europe/Paris`        | Central European Time         |
| **Japan**       | `Asia/Tokyo`          | Japan Standard Time           |
| **China**       | `Asia/Shanghai`       | China Standard Time           |
| **India**       | `Asia/Kolkata`        | India Standard Time           |
| **Australia**   | `Australia/Sydney`    | Australian Eastern Time       |

### Testing Timezone Configuration

Use the config validator to verify timezone settings:

```bash
# Validate timezone configuration
./config-validator -config config.yaml

# Check timezone parsing
./config-validator -config config.yaml | grep "Timezone:"
```
