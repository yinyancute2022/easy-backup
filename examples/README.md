# Docker Compose Example

This example demonstrates the Easy Backup tool in action with a complete environment including:

- **PostgreSQL Database** - With sample tables and data
- **MySQL Database** - With sample tables and data
- **MongoDB Database** - With sample collections and documents
- **MinIO S3 Storage** - S3-compatible object storage for backups
- **Data Generators** - Continuously insert random data into all databases
- **Easy Backup Service** - Performs backups of all databases with different schedules

## Quick Start

### 1. Build the Application

```bash
make build
```

### 2. Setup Environment Variables

Navigate to the examples directory and create a `.env` file with your Slack configuration:

```bash
# Navigate to examples directory
cd examples

# Copy the example environment file
cp .env.example .env

# Edit .env with your actual Slack credentials
# Get your bot token from https://api.slack.com/apps -> OAuth & Permissions -> Bot User OAuth Token
# Get your channel ID from Slack -> Right click on channel -> View channel details -> Copy channel ID
```

Example `.env` file:

```bash
# Database Configuration
DATABASE_URL=postgres://testuser:testpass@postgres:5432/testdb?sslmode=disable
POSTGRES_DATABASE_URL=postgres://testuser:testpass@postgres:5432/testdb?sslmode=disable
MYSQL_DATABASE_URL=mysql://testuser:testpass@mysql:3306/testdb
MONGODB_DATABASE_URL=mongodb://testuser:testpass@mongodb:27017/testdb

# S3/MinIO Configuration
AWS_ACCESS_KEY_ID=minioadmin
AWS_SECRET_ACCESS_KEY=minioadmin123
AWS_REGION=us-east-1
S3_BUCKET=backup-bucket
S3_ENDPOINT=http://minio:9000

# Slack Configuration (required for notifications)
SLACK_BOT_TOKEN=xoxb-your-actual-bot-token-here
SLACK_CHANNEL_ID=C0123456789ABCDEF
```

### 3. Start the Example Environment

```bash
docker compose up -d
```

### 4. Monitor the Services

```bash
# Check all services status
docker compose ps

# View logs from all services
docker compose logs -f

# View logs from specific service
docker compose logs -f easy-backup
```

## Service Details

### PostgreSQL Database

- **Port**: 5432
- **Database**: testdb
- **Username**: testuser
- **Password**: testpass
- **Tables**: users, posts, comments
- **Backup Schedule**: Every 1 minute (using cron: `*/1 * * * *`)
- **Retention**: 10 minutes

### MySQL Database

- **Port**: 3306
- **Database**: testdb
- **Username**: testuser
- **Password**: testpass
- **Tables**: users, posts, comments
- **Backup Schedule**: Every 1 minute (using cron: `*/1 * * * *`)
- **Retention**: 15 minutes

### MongoDB Database

- **Port**: 27017
- **Database**: testdb
- **Username**: testuser
- **Password**: testpass
- **Collections**: users, posts, comments
- **Backup Schedule**: Every 1 minute (using cron: `*/1 * * * *`)
- **Retention**: 20 minutes

### MinIO S3 Storage

- **API Port**: 9000
- **Console Port**: 9001 (Web UI)
- **Access Key**: minioadmin
- **Secret Key**: minioadmin123
- **Bucket**: backup-bucket
- **Console URL**: http://localhost:9001

### Easy Backup Service

- **Health Check**: http://localhost:8080/health
- **Metrics**: http://localhost:8080/metrics
- **Supported Databases**: PostgreSQL, MySQL, MongoDB

### Data Generators

- **PostgreSQL Generator**: Inserts random users, posts, and comments every 30 seconds
- **MySQL Generator**: Inserts random users, posts, and comments every 30 seconds
- **MongoDB Generator**: Inserts random users, posts, and comments every 30 seconds
- All generators create realistic sample data for testing backup functionality

## Accessing Services

### MinIO Web Console

1. Open http://localhost:9001
2. Login with:
   - Username: `minioadmin`
   - Password: `minioadmin123`
3. Navigate to "Buckets" → "backup-bucket" to see backups

### Database Access

```bash
# Connect to PostgreSQL
docker exec -it example-postgres psql -U testuser -d testdb

# View tables
\dt

# Check current data
SELECT COUNT(*) FROM users;
SELECT COUNT(*) FROM posts;
SELECT COUNT(*) FROM comments;
```

```bash
# Connect to MySQL
docker exec -it example-mysql mysql -u testuser -ptestpass testdb

# View tables
SHOW TABLES;

# Check current data
SELECT COUNT(*) FROM users;
SELECT COUNT(*) FROM posts;
SELECT COUNT(*) FROM comments;
```

```bash
# Connect to MongoDB
docker exec -it example-mongodb mongosh -u testuser -p testpass --authenticationDatabase testdb testdb

# View collections
show collections

# Check current data
db.users.countDocuments()
db.posts.countDocuments()
db.comments.countDocuments()
```

### Health Check

```bash
# Check backup service health
curl -s http://localhost:8080/health | jq '.'

# Check Prometheus metrics
curl -s http://localhost:8080/metrics
```

## Configuration Details

The example uses the following configuration:

```yaml
global:
  schedule: "*/1 * * * *" # Backup every minute
  retention: "10m" # Keep backups for 10 minutes
  log_level: "info"

  s3:
    bucket: "backup-bucket"
    endpoint: "http://minio:9000" # MinIO endpoint

strategies:
  - name: "demo-database"
    database_url: "postgres://testuser:testpass@postgres:5432/testdb?sslmode=disable"
    schedule: "*/1 * * * *"
    retention: "10m"
```

## What Happens

1. **Initial Setup**:

   - PostgreSQL starts with sample schema and data
   - MinIO starts and creates the backup bucket
   - Data generator begins inserting random data

2. **Backup Process**:

   - Easy Backup service starts and schedules backups every minute
   - Each backup:
     - Connects to PostgreSQL
     - Runs `pg_dump` to create backup
     - Compresses with gzip
     - Uploads to MinIO
     - Logs progress and metrics

3. **Retention Management**:

   - Every backup run cleans up files older than 10 minutes
   - You'll see approximately 10 backup files at any time

4. **Monitoring**:
   - Health endpoint shows backup status
   - Metrics endpoint provides Prometheus metrics
   - Logs show detailed backup operations

## Viewing Backup Files

### In MinIO Console

1. Go to http://localhost:9001
2. Login with minioadmin/minioadmin123
3. Browse to "backup-bucket" → "demo-backups" → "demo-database"
4. See backup files organized by date

### Via Command Line

```bash
# List backup files in MinIO
docker exec example-minio mc ls myminio/backup-bucket/demo-backups/demo-database/ --recursive
```

### Download a Backup

```bash
# Download latest backup
docker exec example-minio mc cp myminio/backup-bucket/demo-backups/demo-database/$(date +%Y/%m/%d)/demo-database_$(date +%Y%m%d)*.sql.gz ./
```

## Testing Backup Restoration

```bash
# 1. Get a backup file
BACKUP_FILE=$(docker exec example-minio mc ls myminio/backup-bucket/demo-backups/demo-database/ --recursive | tail -1 | awk '{print $NF}')

# 2. Download the backup
docker exec example-minio mc cp "myminio/backup-bucket/demo-backups/demo-database/$BACKUP_FILE" /tmp/test-backup.sql.gz

# 3. Create a test database
docker exec example-postgres createdb -U testuser testdb_restore

# 4. Restore the backup
docker exec example-postgres sh -c "gunzip -c /tmp/test-backup.sql.gz | psql -U testuser -d testdb_restore"

# 5. Verify restoration
docker exec example-postgres psql -U testuser -d testdb_restore -c "SELECT COUNT(*) FROM users;"
```

## Scaling the Example

### Multiple Databases

Add more database services and strategies:

```yaml
# Add to docker-compose.yml
postgres2:
  image: postgres:15-alpine
  environment:
    POSTGRES_DB: testdb2
    POSTGRES_USER: testuser2
    POSTGRES_PASSWORD: testpass2

# Add to backup-config.yaml
strategies:
  - name: "demo-database"
    database_url: "postgres://testuser:testpass@postgres:5432/testdb?sslmode=disable"
  - name: "demo-database2"
    database_url: "postgres://testuser2:testpass2@postgres2:5432/testdb2?sslmode=disable"
```

### Different Schedules

Configure different backup frequencies:

```yaml
strategies:
  - name: "critical-db"
    schedule: "*/1 * * * *" # Every minute (for demo purposes)
    retention: "5m" # 5 minute retention
  - name: "regular-db"
    schedule: "*/5 * * * *" # Every 5 minutes
    retention: "30m" # 30 minute retention
```

## Cleanup

```bash
# Stop all services
docker compose down

# Remove volumes (deletes all data)
docker compose down -v

# Remove images
docker compose down --rmi all
```

## Troubleshooting

### Service Won't Start

```bash
# Check service logs
docker compose logs [service-name]

# Check service status
docker compose ps
```

### Backup Failures

```bash
# Check backup service logs
docker compose logs easy-backup

# Check database connectivity
docker exec example-easy-backup pg_isready -h postgres -p 5432 -U testuser

# Check MinIO connectivity
docker exec example-easy-backup wget -q --spider http://minio:9000
```

### Database Issues

```bash
# Check PostgreSQL logs
docker compose logs postgres

# Check database size
docker exec example-postgres psql -U testuser -d testdb -c "SELECT pg_size_pretty(pg_database_size('testdb'));"
```

### Storage Issues

```bash
# Check MinIO status
docker compose logs minio

# List all buckets
docker exec example-minio mc ls myminio/

# Check bucket policy
docker exec example-minio mc anonymous list myminio/backup-bucket
```

## Performance Considerations

For production-like testing:

1. **Increase Data Volume**:

   - Modify `generate-data.sh` to create more data
   - Add more complex queries and operations

2. **Test Backup Performance**:

   - Monitor backup duration in logs
   - Check resource usage with `docker stats`

3. **Storage Monitoring**:

   - Monitor MinIO storage usage
   - Test backup/restore times with larger datasets

4. **Network Simulation**:
   - Add network delays to simulate real-world conditions
   - Test backup reliability under network stress
