#!/bin/bash

# MySQL Data Generator - Inserts random data into MySQL database

set -e

# Database connection parameters
MYSQL_CMD="mysql -h${MYSQL_HOST} -P${MYSQL_PORT} -u${MYSQL_USER} -p${MYSQL_PASSWORD} ${MYSQL_DATABASE}"

# Arrays for random data generation
FIRST_NAMES=("Alice" "Bob" "Charlie" "Diana" "Eve" "Frank" "Grace" "Henry" "Iris" "Jack")
LAST_NAMES=("Anderson" "Brown" "Clark" "Davis" "Evans" "Ford" "Green" "Harris" "Johnson" "King")
TITLES=("Amazing MySQL Tips" "Database Optimization Guide" "SQL Best Practices" "Performance Tuning" "Advanced Queries" "Data Modeling" "Index Strategies" "Backup Solutions" "Security Guide" "Migration Tips")
STATUSES=("draft" "published" "archived")

# Function to get random element from array
get_random_element() {
  local array=("$@")
  local index=$((RANDOM % ${#array[@]}))
  echo "${array[$index]}"
}

# Function to generate random user
generate_user() {
  local first_name=$(get_random_element "${FIRST_NAMES[@]}")
  local last_name=$(get_random_element "${LAST_NAMES[@]}")
  local username="${first_name,,}_${last_name,,}_$((RANDOM % 1000))"
  local email="${username}@example.com"

  echo "INSERT IGNORE INTO users (username, email, first_name, last_name) VALUES ('$username', '$email', '$first_name', '$last_name');"
}

# Function to generate random post
generate_post() {
  local user_count=$($MYSQL_CMD -e "SELECT COUNT(*) FROM users;" -sN)
  if [ "$user_count" -eq 0 ]; then
    return
  fi

  local user_id=$($MYSQL_CMD -e "SELECT id FROM users ORDER BY RAND() LIMIT 1;" -sN)
  local title=$(get_random_element "${TITLES[@]}")
  local content="This is automatically generated content for the post titled '$title'. It contains useful information about the topic and provides valuable insights for readers. Generated at $(date)."
  local status=$(get_random_element "${STATUSES[@]}")

  echo "INSERT INTO posts (user_id, title, content, status) VALUES ($user_id, '$title - $(date +%s)', '$content', '$status');"
}

# Function to generate random comment
generate_comment() {
  local user_count=$($MYSQL_CMD -e "SELECT COUNT(*) FROM users;" -sN)
  local post_count=$($MYSQL_CMD -e "SELECT COUNT(*) FROM posts;" -sN)

  if [ "$user_count" -eq 0 ] || [ "$post_count" -eq 0 ]; then
    return
  fi

  local user_id=$($MYSQL_CMD -e "SELECT id FROM users ORDER BY RAND() LIMIT 1;" -sN)
  local post_id=$($MYSQL_CMD -e "SELECT id FROM posts ORDER BY RAND() LIMIT 1;" -sN)
  local content="This is an automatically generated comment. Posted at $(date). Very insightful post!"

  echo "INSERT INTO comments (post_id, user_id, content) VALUES ($post_id, $user_id, '$content');"
}

echo "Starting MySQL data generation at $(date)"

# Test connection
if ! $MYSQL_CMD -e "SELECT 1;" >/dev/null 2>&1; then
  echo "Error: Cannot connect to MySQL database"
  exit 1
fi

# Generate random data
for i in {1..3}; do
  # Generate users (30% chance)
  if [ $((RANDOM % 10)) -lt 3 ]; then
    SQL=$(generate_user)
    echo "Executing: $SQL"
    $MYSQL_CMD -e "$SQL"
  fi

  # Generate posts (50% chance)
  if [ $((RANDOM % 10)) -lt 5 ]; then
    SQL=$(generate_post)
    if [ ! -z "$SQL" ]; then
      echo "Executing: $SQL"
      $MYSQL_CMD -e "$SQL"
    fi
  fi

  # Generate comments (70% chance)
  if [ $((RANDOM % 10)) -lt 7 ]; then
    SQL=$(generate_comment)
    if [ ! -z "$SQL" ]; then
      echo "Executing: $SQL"
      $MYSQL_CMD -e "$SQL"
    fi
  fi
done

# Show current stats
echo "Current database stats:"
$MYSQL_CMD -e "
SELECT
    'users' as table_name,
    COUNT(*) as record_count
FROM users
UNION ALL
SELECT
    'posts' as table_name,
    COUNT(*) as record_count
FROM posts
UNION ALL
SELECT
    'comments' as table_name,
    COUNT(*) as record_count
FROM comments;
"

echo "MySQL data generation completed at $(date)"
