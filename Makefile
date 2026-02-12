.PHONY: all build test clean lint fmt vet run docker-build help

# Build variables
BINARY_NAME := llm-secret-interceptor
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date +%s)
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Go variables
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOVET := $(GOCMD) vet
GOFMT := gofmt
GOLINT := golangci-lint

# Tool versions - managed by Renovate
GOLANGCI_LINT_VERSION := v2.1.6
GOSEC_VERSION := v2.22.0
GOVULNCHECK_VERSION := v1.1.4
GOCYCLO_VERSION := v0.6.0

# Directories
CMD_DIR := ./cmd/proxy
BIN_DIR := ./bin
COVERAGE_DIR := ./coverage

# Default target
all: lint test build

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_DIR)

## build-linux: Build for Linux (for Docker)
build-linux:
	@echo "Building $(BINARY_NAME) for Linux..."
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)

## run: Run the proxy
run: build
	$(BIN_DIR)/$(BINARY_NAME)

## test: Run all tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

## test-unit: Run unit tests only
test-unit:
	@echo "Running unit tests..."
	$(GOTEST) -v -race -short ./...

## test-unit-coverage: Run unit tests with coverage
test-unit-coverage:
	@echo "Running unit tests with coverage..."
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -v -race -short -coverprofile=$(COVERAGE_DIR)/unit.out ./...
	$(GOCMD) tool cover -func=$(COVERAGE_DIR)/unit.out

## test-integration: Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -race -run Integration ./...

## test-integration-coverage: Run integration tests with coverage
test-integration-coverage:
	@echo "Running integration tests with coverage..."
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -v -race -run Integration -coverprofile=$(COVERAGE_DIR)/integration.out ./...

## coverage: Generate coverage report
coverage: test-unit-coverage
	$(GOCMD) tool cover -html=$(COVERAGE_DIR)/unit.out -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report generated: $(COVERAGE_DIR)/coverage.html"

## lint: Run linter
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	golangci-lint run ./...

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

## gosec: Run security scanner
gosec:
	@echo "Running GoSec security scanner..."
	@which gosec > /dev/null || go install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)
	gosec -exclude-generated ./...

## vuln: Run vulnerability check
vuln:
	@echo "Running vulnerability check..."
	@which govulncheck > /dev/null || go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	govulncheck ./...

## cyclo: Check cyclomatic complexity
cyclo:
	@echo "Checking cyclomatic complexity..."
	@which gocyclo > /dev/null || go install github.com/fzipp/gocyclo/cmd/gocyclo@$(GOCYCLO_VERSION)
	gocyclo -over 15 .

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	rm -rf $(COVERAGE_DIR)
	rm -f coverage.out

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOCMD) mod download
	$(GOCMD) mod tidy

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(BINARY_NAME):$(VERSION) \
		-t $(BINARY_NAME):latest \
		.

## docker-run: Run Docker container
docker-run: docker-build
	docker run -it --rm \
		-p 8080:8080 \
		-p 9090:9090 \
		-v $(PWD)/certs:/app/certs \
		-v $(PWD)/configs/config.yaml:/app/configs/config.yaml:ro \
		$(BINARY_NAME):latest

## docker-compose-up: Start with Docker Compose
docker-compose-up:
	@echo "Starting with Docker Compose..."
	VERSION=$(VERSION) GIT_COMMIT=$(GIT_COMMIT) BUILD_TIME=$(BUILD_TIME) \
		docker compose up --build -d

## docker-compose-down: Stop Docker Compose
docker-compose-down:
	@echo "Stopping Docker Compose..."
	docker compose down

## docker-compose-logs: View Docker Compose logs
docker-compose-logs:
	docker compose logs -f

## generate-ca: Generate self-signed CA certificate
generate-ca:
	@echo "Generating CA certificate..."
	@mkdir -p certs
	$(BIN_DIR)/$(BINARY_NAME) generate-ca certs/ca.crt certs/ca.key
	@echo "CA certificate generated in certs/"

## install-ca-macos: Install CA certificate on macOS
install-ca-macos:
	@echo "Installing CA certificate on macOS..."
	sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain certs/ca.crt
	@echo "CA certificate installed"

## install-ca-linux: Install CA certificate on Linux
install-ca-linux:
	@echo "Installing CA certificate on Linux..."
	sudo cp certs/ca.crt /usr/local/share/ca-certificates/llm-secret-interceptor.crt
	sudo update-ca-certificates
	@echo "CA certificate installed"
