#!/bin/bash

# Test script for Docker Compose example
set -e

echo "🚀 Starting Easy Backup Docker Compose Example Test"

# Check prerequisites
echo "📋 Checking prerequisites..."
if ! command -v docker &>/dev/null; then
  echo "❌ docker not found. Please install Docker."
  exit 1
fi

if ! docker info &>/dev/null; then
  echo "❌ Docker daemon not running. Please start Docker."
  exit 1
fi

# Build the application
echo "🔨 Building application..."
make build

# Start the environment
echo "🐳 Starting Docker Compose environment..."
docker compose up -d

# Wait for services to be ready
echo "⏳ Waiting for services to be ready..."
sleep 30

# Check service status
echo "📊 Checking service status..."
docker compose ps

# Test health endpoints
echo "🏥 Testing health endpoints..."
if curl -f -s http://localhost:8080/health >/dev/null; then
  echo "✅ Health check endpoint is working"
else
  echo "❌ Health check endpoint failed"
  docker compose logs easy-backup
fi

# Test metrics endpoint
if curl -f -s http://localhost:8080/metrics >/dev/null; then
  echo "✅ Metrics endpoint is working"
else
  echo "❌ Metrics endpoint failed"
fi

# Test MinIO
if curl -f -s http://localhost:9000/minio/health/live >/dev/null; then
  echo "✅ MinIO is working"
else
  echo "❌ MinIO failed"
  docker compose logs minio
fi

# Test PostgreSQL
if docker exec example-postgres pg_isready -U testuser -d testdb >/dev/null 2>&1; then
  echo "✅ PostgreSQL is working"
else
  echo "❌ PostgreSQL failed"
  docker compose logs postgres
fi

# Test MySQL
if docker exec example-mysql mysqladmin ping -h localhost -u testuser -ptestpass >/dev/null 2>&1; then
  echo "✅ MySQL is working"
else
  echo "❌ MySQL failed"
  docker compose logs mysql
fi

# Test MongoDB
if docker exec example-mongodb mongosh --eval "db.adminCommand('ping')" >/dev/null 2>&1; then
  echo "✅ MongoDB is working"
else
  echo "❌ MongoDB failed"
  docker compose logs mongodb
fi

# Check for backup files (wait a bit for first backup)
echo "⏳ Waiting for first backup to complete..."
sleep 90

echo "📁 Checking for backup files..."
if docker exec example-minio mc ls myminio/backup-bucket/demo-backups/ --recursive 2>/dev/null | grep -E "\.(sql\.gz|tar\.gz)$"; then
  echo "✅ Backup files found in MinIO"
  docker exec example-minio mc ls myminio/backup-bucket/demo-backups/ --recursive
else
  echo "❌ No backup files found"
  docker compose logs easy-backup
fi

# Show current health status
echo "🏥 Current health status:"
curl -s http://localhost:8080/health | jq '.' 2>/dev/null || curl -s http://localhost:8080/health

# Show database stats
echo "📊 Database stats:"

echo "PostgreSQL:"
docker exec example-postgres psql -U testuser -d testdb -c "
SELECT
    'users' as table_name, COUNT(*) as count FROM users
UNION ALL
SELECT
    'posts' as table_name, COUNT(*) as count FROM posts
UNION ALL
SELECT
    'comments' as table_name, COUNT(*) as count FROM comments
ORDER BY table_name;
"

echo "MySQL:"
docker exec example-mysql mysql -u testuser -ptestpass testdb -e "
SELECT 'users' as table_name, COUNT(*) as count FROM users
UNION ALL
SELECT 'posts' as table_name, COUNT(*) as count FROM posts
UNION ALL
SELECT 'comments' as table_name, COUNT(*) as count FROM comments
ORDER BY table_name;
"

echo "MongoDB:"
docker exec example-mongodb mongosh -u testuser -p testpass --authenticationDatabase testdb testdb --eval "
print('users: ' + db.users.countDocuments());
print('posts: ' + db.posts.countDocuments());
print('comments: ' + db.comments.countDocuments());
"

echo "🎉 Example test completed successfully!"
echo ""
echo "📋 Next steps:"
echo "   - View MinIO console: http://localhost:9001 (minioadmin/minioadmin123)"
echo "   - Check health: curl http://localhost:8080/health"
echo "   - View metrics: curl http://localhost:8080/metrics"
echo "   - View logs: docker compose logs -f"
echo "   - Cleanup: docker compose down -v"
