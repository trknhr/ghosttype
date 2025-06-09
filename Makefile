export CGO_ENABLED := 1
export MACOSX_DEPLOYMENT_TARGET := 15.2
export CGO_LDFLAGS := -mmacosx-version-min=15.2

APP_NAME := ghosttype
SRC_DIRS := cmd history marcov
BUILD_DIR := ./bin
GO := go
HISTORY_FILE ?= ~/.zsh_history
OUTPUT_DIR ?= ./testdata
EVAL_COUNT ?= 500
MIN_FREQ ?= 2

.PHONY: all build run clean install fmt lint test dev cover-html generate-eval help

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
	$(GO) test -cover ./...

cover-html:
	go test -coverprofile=cover.out ./...
	go tool cover -html=cover.out

dev:
	@echo "🛠  Running in development mode..."
	GOFLAGS="" \
	GHOSTTYPE_LOG_LEVEL=debug \
	go run main.go  $(ARGS) $(ARGS2)

# Generate evaluation data
generate-eval: ## Generate eval data from history file
	@echo "🚀 Generating eval data..."
	@mkdir -p $(OUTPUT_DIR)
	go run main.go generate eval \
		--history $(HISTORY_FILE) \
		--output $(OUTPUT_DIR)/eval_auto.jsonl \
		--count $(EVAL_COUNT) \
		--min-freq $(MIN_FREQ)

# Run evaluations
run-eval: ## Run evaluation with auto-generated data
	@echo "📊 Running evaluation with auto-generated data..."
	go run main.go eval --model prefix --file $(OUTPUT_DIR)/eval.csv

run-eval-all: ## Run evaluation with all models
	@echo "📊 Running comprehensive evaluation..."
	@for model in freq embedding llm; do \
		echo "Testing with $$model model..."; \
		go run main.go eval --model $$model --file $(OUTPUT_DIR)/eval.csv; \
		echo ""; \
	done

# Batch evaluation - test all models at once
run-batch-eval: ## Run evaluation with all models in one go
	@echo "🚀 Running batch evaluation..."
	go run main.go batch-eval --file $(OUTPUT_DIR)/eval_balanced.csv

# # Quick evaluation with fewer cases
# run-quick-eval: ## Quick evaluation with 100 cases
# 	@echo "⚡ Running quick evaluation..."
# 	@make generate-eval EVAL_COUNT=100
# 	go run main.go batch-eval --file $(OUTPUT_DIR)/eval_auto.jsonl

# Compare specific models
compare-models: ## Compare specific models (usage: make compare-models MODELS=freq,embedding)
	@echo "🔄 Comparing models: $(MODELS)"
	go run main.go batch-eval --file $(OUTPUT_DIR)/eval.csv --models $(MODELS)

benchmark: ## Run full benchmark comparison
	@echo "🏁 Running full benchmark..."
	@make generate-eval EVAL_COUNT=1000
	@echo "Results with 1000 test cases:"
	@make run-eval-all
	
compare-tools: ## Compare with existing command-line tools (requires fzf)
	@echo "⚔️  Comparing with existing tools..."
	@if command -v fzf >/dev/null 2>&1; then \
		echo "Testing fzf performance..."; \
		time (head -100 $(HISTORY_FILE) | fzf -f "git p" | head -1); \
	else \
		echo "fzf not installed, skipping comparison"; \
	fi
	@echo "Testing ghosttype performance..."
	@time ./bin/ghosttype "git p" --quick-exit

generate-balanced: ## Generate balanced evaluation dataset
	@echo "🎯 Generating balanced evaluation dataset..."
	go run main.go generate balanced \
		--output $(OUTPUT_DIR)/eval_balanced.csv \
		--count 1000 \
		--history $(HISTORY_FILE)

extract-github: ## Extract commands from GitHub
	@echo "🔍 Extracting commands from GitHub..."
	go run main.go generate github-extract \
		--token $(GITHUB_TOKEN) \
		--output $(OUTPUT_DIR)/github_commands.json \
		--languages go,python,javascript,rust,bash,zsh \
		--max-repos 50 \
		--cache-dir ./github_cache

# Ensemble evaluation (production-like)
run-ensemble-eval: ## Run ensemble evaluation (mimics production)
	@echo "🎯 Running ensemble evaluation..."
	go run main.go ensemble-eval --file $(OUTPUT_DIR)/eval_balanced.csv

# Quick evaluation for development
run-quick-eval: ## Quick evaluation for development
	@echo "⚡ Running quick evaluation..."
	go run main.go quick-eval --file $(OUTPUT_DIR)/eval_balanced.csv --sample 100

# Comprehensive evaluation pipeline
run-full-eval: ## Run complete evaluation pipeline
	@echo "🚀 Running comprehensive evaluation..."
	@make generate-balanced EVAL_COUNT=500
	@echo "\n1️⃣  Quick check..."
	@make run-quick-eval
	@echo "\n2️⃣  Full ensemble evaluation..."
	@make run-ensemble-eval
	@echo "\n3️⃣  Individual model comparison..."
	@make run-batch-eval

# FZF Benchmark targets
run-benchmark: ## Benchmark against fzf, zoxide, etc.
	@echo "🏁 Running benchmark comparison..."
	go run main.go benchmark --file $(OUTPUT_DIR)/eval_balanced.csv

run-quick-benchmark: ## Quick benchmark comparison (50 cases)
	@echo "⚡ Running quick benchmark..."
	go run main.go benchmark --file $(OUTPUT_DIR)/eval_balanced.csv --max-cases 50

benchmark-fzf: ## Benchmark against fzf only
	@echo "🔍 Benchmarking against fzf..."
	go run main.go benchmark --file $(OUTPUT_DIR)/eval_balanced.csv --tools fzf

benchmark-ghosttype-vs-fzf: ## Direct ghosttype vs fzf comparison
	@echo "⚔️  Ghosttype vs FZF showdown..."
	go run main.go benchmark --file $(OUTPUT_DIR)/eval_balanced.csv --tools ghosttype,fzf --max-cases 100

# Complete evaluation pipeline with benchmark
run-complete-eval: ## Run complete evaluation pipeline
	@echo "🚀 Running complete evaluation pipeline..."
	@make generate-balanced EVAL_COUNT=500
	@echo "\n1️⃣  Quick ensemble check..."
	@make run-quick-eval
	@echo "\n2️⃣  FZF comparison..."
	@make benchmark-ghosttype-vs-fzf
	@echo "\n3️⃣  Full ensemble evaluation..."
	@make run-ensemble-eval
	@echo "\n4️⃣  Individual model breakdown..."
	@make run-batch-eval

# Benchmark with different data sizes
benchmark-small: ## Small benchmark (50 cases)
	@echo "📊 Small benchmark..."
	go run main.go benchmark --file $(OUTPUT_DIR)/eval_balanced.csv --max-cases 50 --tools ghosttype,fzf

benchmark-medium: ## Medium benchmark (200 cases)
	@echo "📊 Medium benchmark..."
	go run main.go benchmark --file $(OUTPUT_DIR)/eval_balanced.csv --max-cases 200 --tools ghosttype,fzf

benchmark-large: ## Large benchmark (all cases)
	@echo "📊 Large benchmark..."
	go run main.go benchmark --file $(OUTPUT_DIR)/eval_balanced.csv --tools ghosttype,fzf

# Benchmark with memory profiling
benchmark-with-memory: ## Benchmark with memory usage tracking
	@echo "💾 Benchmark with memory profiling..."
	go run main.go benchmark --file $(OUTPUT_DIR)/eval_balanced.csv --tools ghosttype,fzf --memory

# Export benchmark results for analysis
export-benchmark: ## Export benchmark results to CSV
	@echo "📤 Exporting benchmark results..."
	go run main.go benchmark --file $(OUTPUT_DIR)/eval_balanced.csv --tools ghosttype,fzf --output benchmark_results.json
	@echo "Results saved to benchmark_results.json"

# Helper targets
check-fzf: ## Check if fzf is installed
	@if command -v fzf >/dev/null 2>&1; then \
		echo "✅ fzf found: $$(fzf --version)"; \
	else \
		echo "❌ fzf not found. Install with: brew install fzf"; \
		exit 1; \
	fi

install-benchmark-deps: ## Install benchmark dependencies
	@echo "📦 Installing benchmark dependencies..."
	@if ! command -v fzf >/dev/null 2>&1; then \
		echo "Installing fzf..."; \
		brew install fzf || (echo "Please install fzf manually"); \
	fi
	@if ! command -v zoxide >/dev/null 2>&1; then \
		echo "Installing zoxide..."; \
		brew install zoxide || (echo "zoxide installation failed"); \
	fi
	@echo "✅ Dependencies check complete"

# Quick comparison for demos
demo-comparison: ## Quick demo comparison
	@echo "🎬 Demo: Ghosttype vs FZF"
	@echo "════════════════════════════"
	@make check-fzf
	@make benchmark-small

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
