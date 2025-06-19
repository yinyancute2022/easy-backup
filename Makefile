# Makefile for Easy Backup
# Project variables
BINARY_NAME=easy-backup
CONFIG_VALIDATOR=config-validator
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Directories
DIST_DIR=dist
CMD_DIR=cmd
INTERNAL_DIR=internal
SCRIPTS_DIR=scripts

# Go related variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Docker variables
DOCKER_IMAGE=ghcr.io/$(shell git config --get remote.origin.url | sed 's/.*github.com\///; s/\.git$$//')/easy-backup
DOCKER_TAG?=latest

# Default target
.PHONY: all
all: clean deps test build

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting Go code..."
	$(GOFMT) ./...

# Vet code
.PHONY: vet
vet:
	@echo "Vetting Go code..."
	$(GOVET) ./...

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

# Run tests with coverage report
.PHONY: test-coverage
test-coverage: test
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with short flag
.PHONY: test-short
test-short:
	@echo "Running short tests..."
	$(GOTEST) -short ./...

# Build for current platform
.PHONY: build
build: clean-dist
	@echo "Building $(BINARY_NAME) for current platform..."
	@mkdir -p $(DIST_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME) ./$(CMD_DIR)/$(BINARY_NAME)
	$(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(CONFIG_VALIDATOR) ./$(CMD_DIR)/$(CONFIG_VALIDATOR)

# Build for all platforms
.PHONY: build-all
build-all: clean-dist
	@echo "Building $(BINARY_NAME) for all platforms..."
	@mkdir -p $(DIST_DIR)

	# Linux AMD64
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./$(CMD_DIR)/$(BINARY_NAME)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(CONFIG_VALIDATOR)-linux-amd64 ./$(CMD_DIR)/$(CONFIG_VALIDATOR)

	# Linux ARM64
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 ./$(CMD_DIR)/$(BINARY_NAME)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(CONFIG_VALIDATOR)-linux-arm64 ./$(CMD_DIR)/$(CONFIG_VALIDATOR)

	# Darwin AMD64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 ./$(CMD_DIR)/$(BINARY_NAME)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(CONFIG_VALIDATOR)-darwin-amd64 ./$(CMD_DIR)/$(CONFIG_VALIDATOR)

	# Darwin ARM64 (Apple Silicon)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./$(CMD_DIR)/$(BINARY_NAME)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(CONFIG_VALIDATOR)-darwin-arm64 ./$(CMD_DIR)/$(CONFIG_VALIDATOR)

	# Windows AMD64
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe ./$(CMD_DIR)/$(BINARY_NAME)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(CONFIG_VALIDATOR)-windows-amd64.exe ./$(CMD_DIR)/$(CONFIG_VALIDATOR)

	@echo "Build completed! Binaries are in the $(DIST_DIR)/ directory."

# Build Docker image
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest

# Push Docker image
.PHONY: docker-push
docker-push: docker-build
	@echo "Pushing Docker image..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest

# Run the application locally
.PHONY: run
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(DIST_DIR)/$(BINARY_NAME) -config config.example.yaml

# Run config validator
.PHONY: validate-config
validate-config: build
	@echo "Validating configuration..."
	./$(DIST_DIR)/$(CONFIG_VALIDATOR) -config config.example.yaml

# Development server with hot reload (requires air)
.PHONY: dev
dev:
	@echo "Starting development server..."
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "air not found. Install it with: go install github.com/cosmtrek/air@latest"; \
		echo "Falling back to regular run..."; \
		$(MAKE) run; \
	fi

# Clean build artifacts
.PHONY: clean
clean: clean-dist clean-test
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)

# Clean dist directory
.PHONY: clean-dist
clean-dist:
	@echo "Cleaning dist directory..."
	@rm -rf $(DIST_DIR)

# Clean test artifacts
.PHONY: clean-test
clean-test:
	@echo "Cleaning test artifacts..."
	@rm -f coverage.out coverage.html

# Update dependencies
.PHONY: update-deps
update-deps:
	@echo "Updating dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy

# Security check (requires gosec)
.PHONY: security
security:
	@echo "Running security checks..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not found. Install it with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi

# Generate mocks (requires mockgen)
.PHONY: mocks
mocks:
	@echo "Generating mocks..."
	@if command -v mockgen >/dev/null 2>&1; then \
		go generate ./...; \
	else \
		echo "mockgen not found. Install it with: go install github.com/golang/mock/mockgen@latest"; \
	fi

# Release preparation
.PHONY: release
release: clean deps test build-all
	@echo "Release build completed!"
	@echo "Version: $(VERSION)"
	@echo "Build time: $(BUILD_TIME)"
	@ls -la $(DIST_DIR)/

# Install tools
.PHONY: install-tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/cosmtrek/air@latest
	go install github.com/golang/mock/mockgen@latest

# Docker Compose Example
.PHONY: example-up
example-up: build
	@echo "Starting Docker Compose example environment..."
	docker compose -f examples/docker-compose.yml up -d --build

.PHONY: example-down
example-down:
	@echo "Stopping Docker Compose example environment..."
	docker compose -f examples/docker-compose.yml down

.PHONY: example-logs
example-logs:
	@echo "Showing Docker Compose example logs..."
	docker compose -f examples/docker-compose.yml logs -f

.PHONY: example-clean
example-clean:
	@echo "Cleaning Docker Compose example environment..."
	docker compose -f examples/docker-compose.yml down -v --rmi local

.PHONY: example-status
example-status:
	@echo "Docker Compose example status..."
	docker compose -f examples/docker-compose.yml ps

.PHONY: example-test
example-test:
	@echo "Running Docker Compose example test..."
	./examples/test.sh

# Integration test - comprehensive check after changes
.PHONY: integration-test
integration-test: clean
	@echo "=========================================="
	@echo "🧪 Running Integration Test Suite"
	@echo "=========================================="

	@echo "\n📋 Step 1: Installing dependencies..."
	$(MAKE) deps

	@echo "\n🔍 Step 2: Formatting code..."
	$(MAKE) fmt

	@echo "\n🔍 Step 3: Vetting code..."
	$(MAKE) vet

	@echo "\n🧪 Step 4: Running unit tests..."
	$(MAKE) test

	@echo "\n🔨 Step 5: Building for current platform..."
	$(MAKE) build

	@echo "\n✅ Step 6: Testing binary execution..."
	@echo "Testing easy-backup binary..."
	@if ./$(DIST_DIR)/$(BINARY_NAME) -h >/dev/null 2>&1; then \
		echo "✅ easy-backup binary works"; \
	else \
		echo "❌ easy-backup binary failed"; \
		exit 1; \
	fi

	@echo "Testing config-validator binary..."
	@if ./$(DIST_DIR)/$(CONFIG_VALIDATOR) -h >/dev/null 2>&1; then \
		echo "✅ config-validator binary works"; \
	else \
		echo "❌ config-validator binary failed"; \
		exit 1; \
	fi

	@echo "\n📝 Step 7: Validating example configuration..."
	@if [ -f config.example.yaml ]; then \
		./$(DIST_DIR)/$(CONFIG_VALIDATOR) -config config.example.yaml && echo "✅ config.example.yaml is valid"; \
	else \
		echo "⚠️  config.example.yaml not found, skipping validation"; \
	fi

	@echo "\n🐳 Step 8: Testing Docker build..."
	@if command -v docker >/dev/null 2>&1; then \
		$(MAKE) docker-build && echo "✅ Docker build successful"; \
	else \
		echo "⚠️  Docker not found, skipping Docker build test"; \
	fi

	@echo "\n🏗️  Step 9: Building for all platforms..."
	$(MAKE) build-all

	@echo "\n📊 Step 10: Generating test coverage report..."
	$(MAKE) test-coverage

	@echo "\n🔒 Step 11: Running security checks..."
	@if command -v gosec >/dev/null 2>&1; then \
		$(MAKE) security && echo "✅ Security checks passed"; \
	else \
		echo "⚠️  gosec not installed, skipping security checks"; \
		echo "   Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi

	@echo "\n=========================================="
	@echo "🎉 Integration Test Suite Completed!"
	@echo "=========================================="
	@echo "📦 Binaries built for all platforms:"
	@ls -la $(DIST_DIR)/ | grep -E '\.(exe|)$$' || true
	@echo "\n📊 Coverage report: coverage.html"
	@echo "🐳 Docker image: $(DOCKER_IMAGE):$(DOCKER_TAG)"
	@echo "✅ All checks passed! Ready for release."

# Quick integration test (faster version without cross-platform builds)
.PHONY: integration-test-quick
integration-test-quick: clean
	@echo "=========================================="
	@echo "⚡ Running Quick Integration Test"
	@echo "=========================================="

	@echo "\n📋 Installing dependencies..."
	$(MAKE) deps

	@echo "\n🔍 Code quality checks..."
	$(MAKE) fmt vet

	@echo "\n🧪 Running tests..."
	$(MAKE) test

	@echo "\n🔨 Building for current platform..."
	$(MAKE) build

	@echo "\n✅ Testing binaries..."
	@./$(DIST_DIR)/$(BINARY_NAME) -h >/dev/null 2>&1 && echo "✅ easy-backup works"
	@./$(DIST_DIR)/$(CONFIG_VALIDATOR) -h >/dev/null 2>&1 && echo "✅ config-validator works"

	@echo "\n📝 Validating configuration..."
	@if [ -f config.example.yaml ]; then \
		./$(DIST_DIR)/$(CONFIG_VALIDATOR) -config config.example.yaml && echo "✅ Configuration valid"; \
	fi

	@echo "\n🎉 Quick Integration Test Completed!"

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo ""
	@echo "🔧 Build & Test:"
	@echo "  all                - Run clean, deps, test, and build"
	@echo "  deps               - Install dependencies"
	@echo "  fmt                - Format Go code"
	@echo "  vet                - Vet Go code"
	@echo "  test               - Run tests"
	@echo "  test-coverage      - Run tests with coverage report"
	@echo "  test-short         - Run short tests"
	@echo "  build              - Build for current platform"
	@echo "  build-all          - Build for all platforms"
	@echo ""
	@echo "🧪 Integration Testing:"
	@echo "  integration-test       - Full integration test (recommended after changes)"
	@echo "  integration-test-quick - Quick integration test (faster, current platform only)"
	@echo ""
	@echo "🐳 Docker:"
	@echo "  docker-build       - Build Docker image"
	@echo "  docker-push        - Build and push Docker image"
	@echo ""
	@echo "🚀 Run & Validate:"
	@echo "  run                - Run the application locally"
	@echo "  validate-config    - Run config validator"
	@echo "  dev                - Start development server with hot reload"
	@echo ""
	@echo "🧹 Cleanup:"
	@echo "  clean              - Clean build artifacts"
	@echo "  clean-dist         - Clean dist directory"
	@echo "  clean-test         - Clean test artifacts"
	@echo ""
	@echo "🔒 Security & Quality:"
	@echo "  security           - Run security checks"
	@echo "  update-deps        - Update dependencies"
	@echo "  mocks              - Generate mocks"
	@echo ""
	@echo "📦 Release:"
	@echo "  release            - Prepare release build"
	@echo "  install-tools      - Install development tools"
	@echo ""
	@echo "🐳 Examples:"
	@echo "  example-up         - Start Docker Compose example environment"
	@echo "  example-down       - Stop Docker Compose example environment"
	@echo "  example-logs       - Show Docker Compose example logs"
	@echo "  example-clean      - Clean Docker Compose example environment"
	@echo "  example-status     - Show Docker Compose example status"
	@echo "  example-test       - Run Docker Compose example test"
	@echo ""
	@echo "📚 Help:"
	@echo "  help               - Show this help message"
