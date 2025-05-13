export CGO_ENABLED := 1
export MACOSX_DEPLOYMENT_TARGET := 15.2
export CGO_LDFLAGS := -mmacosx-version-min=15.2

APP_NAME := ghosttype
SRC_DIRS := cmd history marcov
BUILD_DIR := ./bin
GO := go

.PHONY: all build run clean install fmt lint test

all: build

build:
	@echo "🔨 Building $(APP_NAME)..."
	$(GO) build -o $(BUILD_DIR)/$(APP_NAME) main.go

run: build
	@echo "🚀 Running $(APP_NAME)..."
	$(BUILD_DIR)/$(APP_NAME)

install:
	@echo "📦 Installing $(APP_NAME)..."
	$(GO) install ./...

clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)

fmt:
	@echo "🎨 Formatting Go code..."
	$(GO_ENV) $(GO) fmt ./...

lint:
	@echo "🔍 Linting..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "golangci-lint not found. Install it via: brew install golangci-lint"; \
	else \
		golangci-lint run ./...; \
	fi

test:
	@echo "🧪 Running tests..."
	$(GO) test ./...

dev:
	@echo "🛠  Running in development mode..."
	GOFLAGS="" \
	GHOSTTYPE_LOG_LEVEL=debug \
	go run main.go $(ARGS)

help:
	@echo ""
	@echo "Available targets:"
	@echo "  make build      - Build the CLI binary"
	@echo "  make run        - Run the CLI"
	@echo "  make install    - Install via go install"
	@echo "  make clean      - Remove build artifacts"
	@echo "  make fmt        - Format Go code"
	@echo "  make lint       - Run golangci-lint (if installed)"
	@echo "  make test       - Run tests"
	@echo ""
