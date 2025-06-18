#!/bin/bash

# MongoDB Data Generator - Inserts random data into MongoDB database

set -e

# MongoDB connection parameters
MONGO_CMD="mongosh mongodb://${MONGO_USER}:${MONGO_PASSWORD}@${MONGO_HOST}:${MONGO_PORT}/${MONGO_DATABASE}"

# Arrays for random data generation
FIRST_NAMES=("Alice" "Bob" "Charlie" "Diana" "Eve" "Frank" "Grace" "Henry" "Iris" "Jack" "Kelly" "Liam" "Mia" "Noah" "Olivia")
LAST_NAMES=("Anderson" "Brown" "Clark" "Davis" "Evans" "Ford" "Green" "Harris" "Johnson" "King" "Lee" "Miller" "Nash" "Owen" "Parker")
LOCATIONS=("New York" "San Francisco" "Seattle" "Chicago" "Boston" "Austin" "Denver" "Portland" "Miami" "Atlanta")
INTERESTS=("technology" "reading" "travel" "photography" "hiking" "cooking" "music" "sports" "gaming" "art" "movies" "fitness")
TITLES=("MongoDB Best Practices" "NoSQL Design Patterns" "Aggregation Pipeline Guide" "Indexing Strategies" "Schema Design" "Performance Optimization" "Data Modeling" "Replication Setup" "Sharding Guide" "Security Tips")
TAGS=("mongodb" "nosql" "database" "javascript" "nodejs" "web" "backend" "development" "tutorial" "guide")

# Function to get random element from array
get_random_element() {
  local array=("$@")
  local index=$((RANDOM % ${#array[@]}))
  echo "${array[$index]}"
}

# Function to get random subset of array
get_random_subset() {
  local array=("$@")
  local count=$((1 + RANDOM % 3)) # 1-3 elements
  local selected=()

  for ((i = 0; i < count; i++)); do
    local element=$(get_random_element "${array[@]}")
    selected+=("\"$element\"")
  done

  echo "[$(
    IFS=,
    echo "${selected[*]}"
  )]"
}

echo "Starting MongoDB data generation at $(date)"

# Test connection
if ! $MONGO_CMD --eval "db.adminCommand('ping')" >/dev/null 2>&1; then
  echo "Error: Cannot connect to MongoDB database"
  exit 1
fi

# Generate random data using MongoDB shell
$MONGO_CMD --eval "
// Generate random users
for (let i = 0; i < 3; i++) {
    if (Math.random() < 0.4) { // 40% chance
        const firstNames = ['Alice', 'Bob', 'Charlie', 'Diana', 'Eve', 'Frank', 'Grace', 'Henry', 'Iris', 'Jack'];
        const lastNames = ['Anderson', 'Brown', 'Clark', 'Davis', 'Evans', 'Ford', 'Green', 'Harris', 'Johnson', 'King'];
        const locations = ['New York', 'San Francisco', 'Seattle', 'Chicago', 'Boston', 'Austin', 'Denver', 'Portland'];
        const interests = ['technology', 'reading', 'travel', 'photography', 'hiking', 'cooking', 'music', 'sports'];

        const firstName = firstNames[Math.floor(Math.random() * firstNames.length)];
        const lastName = lastNames[Math.floor(Math.random() * lastNames.length)];
        const username = firstName.toLowerCase() + '_' + lastName.toLowerCase() + '_' + Math.floor(Math.random() * 1000);
        const email = username + '@example.com';
        const location = locations[Math.floor(Math.random() * locations.length)];
        const userInterests = interests.sort(() => 0.5 - Math.random()).slice(0, 2 + Math.floor(Math.random() * 3));

        try {
            db.users.insertOne({
                username: username,
                email: email,
                firstName: firstName,
                lastName: lastName,
                profile: {
                    age: 20 + Math.floor(Math.random() * 40),
                    location: location,
                    interests: userInterests
                },
                createdAt: new Date(),
                updatedAt: new Date()
            });
            print('Generated user: ' + username);
        } catch (e) {
            print('Error generating user: ' + e.message);
        }
    }
}

// Generate random posts
const userCount = db.users.countDocuments();
if (userCount > 0) {
    for (let i = 0; i < 3; i++) {
        if (Math.random() < 0.6) { // 60% chance
            const titles = ['MongoDB Best Practices', 'NoSQL Design Patterns', 'Aggregation Pipeline Guide', 'Indexing Strategies', 'Schema Design'];
            const tags = ['mongodb', 'nosql', 'database', 'javascript', 'nodejs', 'web', 'backend'];
            const statuses = ['draft', 'published', 'archived'];

            const users = db.users.aggregate([{ \$sample: { size: 1 } }]).toArray();
            if (users.length > 0) {
                const title = titles[Math.floor(Math.random() * titles.length)] + ' - ' + Date.now();
                const content = 'This is automatically generated content for: ' + title + '. Generated at ' + new Date().toISOString();
                const postTags = tags.sort(() => 0.5 - Math.random()).slice(0, 2 + Math.floor(Math.random() * 3));
                const status = statuses[Math.floor(Math.random() * statuses.length)];

                db.posts.insertOne({
                    userId: users[0]._id,
                    title: title,
                    content: content,
                    tags: postTags,
                    status: status,
                    views: Math.floor(Math.random() * 500),
                    likes: Math.floor(Math.random() * 50),
                    createdAt: new Date(),
                    updatedAt: new Date()
                });
                print('Generated post: ' + title);
            }
        }
    }
}

// Generate random comments
const postCount = db.posts.countDocuments();
if (userCount > 0 && postCount > 0) {
    for (let i = 0; i < 5; i++) {
        if (Math.random() < 0.8) { // 80% chance
            const users = db.users.aggregate([{ \$sample: { size: 1 } }]).toArray();
            const posts = db.posts.aggregate([{ \$sample: { size: 1 } }]).toArray();

            if (users.length > 0 && posts.length > 0) {
                const comments = [
                    'Great article! Very informative.',
                    'Thanks for sharing this valuable information.',
                    'This helped me understand the concept better.',
                    'Excellent explanation with good examples.',
                    'Looking forward to more content like this.'
                ];

                const content = comments[Math.floor(Math.random() * comments.length)] + ' Posted at ' + new Date().toISOString();

                db.comments.insertOne({
                    postId: posts[0]._id,
                    userId: users[0]._id,
                    content: content,
                    likes: Math.floor(Math.random() * 20),
                    createdAt: new Date()
                });
                print('Generated comment for post: ' + posts[0].title);
            }
        }
    }
}

// Show current stats
print('\\nCurrent database stats:');
print('Users: ' + db.users.countDocuments());
print('Posts: ' + db.posts.countDocuments());
print('Comments: ' + db.comments.countDocuments());
"

echo "MongoDB data generation completed at $(date)"
