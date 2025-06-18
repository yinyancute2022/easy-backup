// Initialize MongoDB database with sample data
db = db.getSiblingDB('testdb');

// Create collections and insert sample data

// Users collection
db.users.insertMany([
  {
    username: "john_doe",
    email: "john@example.com",
    firstName: "John",
    lastName: "Doe",
    profile: {
      age: 30,
      location: "New York",
      interests: ["technology", "reading", "travel"]
    },
    createdAt: new Date(),
    updatedAt: new Date()
  },
  {
    username: "jane_smith",
    email: "jane@example.com",
    firstName: "Jane",
    lastName: "Smith",
    profile: {
      age: 28,
      location: "San Francisco",
      interests: ["photography", "hiking", "cooking"]
    },
    createdAt: new Date(),
    updatedAt: new Date()
  },
  {
    username: "bob_wilson",
    email: "bob@example.com",
    firstName: "Bob",
    lastName: "Wilson",
    profile: {
      age: 35,
      location: "Seattle",
      interests: ["music", "sports", "gaming"]
    },
    createdAt: new Date(),
    updatedAt: new Date()
  }
]);

// Posts collection
db.posts.insertMany([
  {
    userId: db.users.findOne({ username: "john_doe" })._id,
    title: "Getting Started with MongoDB",
    content: "MongoDB is a powerful NoSQL database...",
    tags: ["mongodb", "nosql", "database"],
    status: "published",
    views: 150,
    likes: 12,
    createdAt: new Date(),
    updatedAt: new Date()
  },
  {
    userId: db.users.findOne({ username: "jane_smith" })._id,
    title: "Building Modern Web Applications",
    content: "Learn how to build scalable web applications...",
    tags: ["web", "javascript", "nodejs"],
    status: "published",
    views: 203,
    likes: 25,
    createdAt: new Date(),
    updatedAt: new Date()
  },
  {
    userId: db.users.findOne({ username: "bob_wilson" })._id,
    title: "Database Design Patterns",
    content: "Explore common patterns for database design...",
    tags: ["design", "patterns", "database"],
    status: "draft",
    views: 0,
    likes: 0,
    createdAt: new Date(),
    updatedAt: new Date()
  }
]);

// Comments collection
db.comments.insertMany([
  {
    postId: db.posts.findOne({ title: "Getting Started with MongoDB" })._id,
    userId: db.users.findOne({ username: "jane_smith" })._id,
    content: "Excellent introduction to MongoDB!",
    likes: 5,
    createdAt: new Date()
  },
  {
    postId: db.posts.findOne({ title: "Getting Started with MongoDB" })._id,
    userId: db.users.findOne({ username: "bob_wilson" })._id,
    content: "Very helpful for beginners. Thanks for sharing!",
    likes: 3,
    createdAt: new Date()
  },
  {
    postId: db.posts.findOne({ title: "Building Modern Web Applications" })._id,
    userId: db.users.findOne({ username: "john_doe" })._id,
    content: "Great insights on web development.",
    likes: 2,
    createdAt: new Date()
  }
]);

// Create indexes for better performance
db.users.createIndex({ "username": 1 }, { unique: true });
db.users.createIndex({ "email": 1 }, { unique: true });
db.posts.createIndex({ "userId": 1 });
db.posts.createIndex({ "status": 1 });
db.posts.createIndex({ "tags": 1 });
db.comments.createIndex({ "postId": 1 });
db.comments.createIndex({ "userId": 1 });

print("MongoDB initialization completed successfully!");
