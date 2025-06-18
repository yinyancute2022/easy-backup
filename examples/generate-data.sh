#!/bin/bash

# Data generator script for PostgreSQL
# This script inserts random data into the test database

set -e

# Array of sample data for random generation
USERNAMES=("user1" "user2" "user3" "user4" "user5" "testuser" "developer" "admin" "guest" "demo")
PRODUCTS=("Laptop" "Desktop" "Mouse" "Keyboard" "Monitor" "Headphones" "Webcam" "Tablet" "Phone" "Charger")
STATUSES=("pending" "processing" "shipped" "completed" "cancelled")

# Function to generate random data
generate_random_user() {
  local username="${USERNAMES[$RANDOM % ${#USERNAMES[@]}]}_$(date +%s)_$RANDOM"
  local email="${username}@example.com"
  local full_name="User $(date +%H%M%S)"

  psql -c "INSERT INTO users (username, email, full_name) VALUES ('$username', '$email', '$full_name') ON CONFLICT (username) DO NOTHING;" 2>/dev/null || true
  echo "Created user: $username"
}

generate_random_order() {
  # Get a random user ID
  local user_id=$(psql -t -c "SELECT id FROM users ORDER BY RANDOM() LIMIT 1;" | tr -d ' ')

  if [ -n "$user_id" ]; then
    local product="${PRODUCTS[$RANDOM % ${#PRODUCTS[@]}]}"
    local quantity=$((RANDOM % 5 + 1))
    local price=$(echo "scale=2; $RANDOM / 100" | bc)
    local status="${STATUSES[$RANDOM % ${#STATUSES[@]}]}"

    psql -c "INSERT INTO orders (user_id, product_name, quantity, price, status) VALUES ($user_id, '$product', $quantity, $price, '$status');" 2>/dev/null || true
    echo "Created order: $product for user $user_id"
  fi
}

update_random_order() {
  # Get a random order ID
  local order_id=$(psql -t -c "SELECT id FROM orders WHERE status IN ('pending', 'processing') ORDER BY RANDOM() LIMIT 1;" | tr -d ' ')

  if [ -n "$order_id" ]; then
    local new_status="${STATUSES[$RANDOM % ${#STATUSES[@]}]}"
    psql -c "UPDATE orders SET status = '$new_status', updated_at = CURRENT_TIMESTAMP WHERE id = $order_id;" 2>/dev/null || true
    echo "Updated order $order_id to status: $new_status"
  fi
}

update_app_setting() {
  local key="last_data_generation"
  local value=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

  psql -c "INSERT INTO app_settings (key, value, description) VALUES ('$key', '$value', 'Last time data was generated') ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP;" 2>/dev/null || true
  echo "Updated app setting: $key = $value"
}

# Main execution
echo "$(date): Starting data generation..."

# Check if database is accessible
if ! psql -c "SELECT 1;" >/dev/null 2>&1; then
  echo "Database not accessible, skipping data generation"
  exit 0
fi

# Generate random operations (weighted towards more inserts)
operation=$((RANDOM % 10))

case $operation in
0 | 1 | 2 | 3 | 4)
  generate_random_order
  ;;
5 | 6)
  generate_random_user
  ;;
7 | 8)
  update_random_order
  ;;
9)
  update_app_setting
  ;;
esac

# Show current database stats
echo "Current database stats:"
psql -c "
SELECT
    'users' as table_name, COUNT(*) as count FROM users
UNION ALL
SELECT
    'orders' as table_name, COUNT(*) as count FROM orders
UNION ALL
SELECT
    'audit_log' as table_name, COUNT(*) as count FROM audit_log
ORDER BY table_name;
"

echo "$(date): Data generation completed"
