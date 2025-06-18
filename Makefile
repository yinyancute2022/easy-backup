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
all: clean deps lint test build

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

# Lint code (requires golangci-lint)
.PHONY: lint
lint:
	@echo "Linting Go code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

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
release: clean deps lint test build-all
	@echo "Release build completed!"
	@echo "Version: $(VERSION)"
	@echo "Build time: $(BUILD_TIME)"
	@ls -la $(DIST_DIR)/

# Install tools
.PHONY: install-tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
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

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all           - Run clean, deps, lint, test, and build"
	@echo "  deps          - Install dependencies"
	@echo "  fmt           - Format Go code"
	@echo "  vet           - Vet Go code"
	@echo "  lint          - Lint Go code (requires golangci-lint)"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  test-short    - Run short tests"
	@echo "  build         - Build for current platform"
	@echo "  build-all     - Build for all platforms"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-push   - Build and push Docker image"
	@echo "  run           - Run the application locally"
	@echo "  validate-config - Run config validator"
	@echo "  dev           - Start development server with hot reload"
	@echo "  clean         - Clean build artifacts"
	@echo "  clean-dist    - Clean dist directory"
	@echo "  clean-test    - Clean test artifacts"
	@echo "  update-deps   - Update dependencies"
	@echo "  security      - Run security checks"
	@echo "  mocks         - Generate mocks"
	@echo "  release       - Prepare release build"
	@echo "  install-tools - Install development tools"
	@echo "  example-up    - Start Docker Compose example environment"
	@echo "  example-down  - Stop Docker Compose example environment"
	@echo "  example-logs  - Show Docker Compose example logs"
	@echo "  example-clean - Clean Docker Compose example environment"
	@echo "  example-status - Show Docker Compose example status"
	@echo "  example-test  - Run Docker Compose example test"
	@echo "  help          - Show this help message"
