.PHONY: all build test lint clean fmt proto migrate-up migrate-down docker-build help

# Variables
MODULE := github.com/Adithya-Monish-Kumar-K/searchplatform
SERVICES := gateway ingestion indexer searcher analytics auth
BIN_DIR := bin
GO := go
GOFLAGS := -trimpath
LDFLAGS := -s -w -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Default target
all: lint test build

# Build all services
build: $(SERVICES)

$(SERVICES):
    @echo "Building $@..."
    $(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$@ ./cmd/$@

# Run tests
test:
    $(GO) test -race -coverprofile=coverage.out ./...
    $(GO) tool cover -func=coverage.out | tail -1

# Run tests with verbose output
test-verbose:
    $(GO) test -race -v -coverprofile=coverage.out ./...

# Integration tests (requires running infrastructure)
test-integration:
    $(GO) test -race -tags=integration -coverprofile=coverage.out ./test/integration/...

# Lint
lint:
    golangci-lint run ./...

# Format code
fmt:
    $(GO) fmt ./...
    goimports -w .

# Clean build artifacts
clean:
    rm -rf $(BIN_DIR) coverage.out coverage.html

# Generate protobuf code
proto:
    ./scripts/generate-proto.sh

# Database migrations
migrate-up:
    migrate -path migrations/postgres -database "$(SP_POSTGRES_DSN)" up

migrate-down:
    migrate -path migrations/postgres -database "$(SP_POSTGRES_DSN)" down 1

# Docker
docker-build:
    @for svc in $(SERVICES); do \
        echo "Building Docker image for $$svc..."; \
        docker build -f deployments/docker/Dockerfile.$$svc -t searchplatform-$$svc:latest .; \
    done

# Show help
help:
    @echo "Available targets:"
    @echo "  all             - Lint, test, and build all services"
    @echo "  build           - Build all service binaries"
    @echo "  <service>       - Build a specific service (e.g., make gateway)"
    @echo "  test            - Run unit tests with race detection"
    @echo "  test-verbose    - Run tests with verbose output"
    @echo "  test-integration- Run integration tests"
    @echo "  lint            - Run golangci-lint"
    @echo "  fmt             - Format all Go code"
    @echo "  clean           - Remove build artifacts"
    @echo "  proto           - Generate protobuf code"
    @echo "  migrate-up      - Run database migrations"
    @echo "  migrate-down    - Roll back last migration"
    @echo "  docker-build    - Build all Docker images"