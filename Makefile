export CGO_ENABLED := 1

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
	$(GO) install -ldflags="-s -w" ./...

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

# Batch evaluation - test all models at once
run-batch-eval: ## Run evaluation with all models in one go
	@echo "🚀 Running batch evaluation..."
	go run main.go batch-eval --file $(OUTPUT_DIR)/eval_balanced.csv
	
compare-launch-time-fzf:
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

# Ensemble evaluation (production-like)
run-ensemble-eval: ## Run ensemble evaluation (mimics production)
	@echo "🎯 Running ensemble evaluation..."
	go run main.go ensemble-eval --file $(OUTPUT_DIR)/eval_balanced.csv

# FZF Benchmark targets
run-benchmark: ## Benchmark against fzf, zoxide, etc.
	@echo "🏁 Running benchmark comparison..."
	go run main.go benchmark --file $(OUTPUT_DIR)/eval_balanced.csv

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

# Quick comparison for demos
demo-comparison: ## Quick demo comparison
	@echo "🎬 Demo: Ghosttype vs FZF"
	@echo "════════════════════════════"
	@make check-fzf
	@make run-benchmark

# Performance profiling targets
profile-cpu: ## Profile CPU usage during predictions
	@echo "🔍 CPU profiling..."
	go run main.go profile cpu --input "git st" --iterations 100 --output cpu.prof

profile-memory: ## Profile memory allocation
	@echo "💾 Memory profiling..."
	go run main.go profile memory --input "docker run" --iterations 50 --output memory.prof

profile-ensemble: ## Profile ensemble model performance
	@echo "🎭 Ensemble profiling..."
	go run main.go profile ensemble --file $(OUTPUT_DIR)/eval_balanced.csv --cases 20 --output ensemble.prof

profile-quick: ## Quick performance check
	@echo "⚡ Quick profiling..."
	go run main.go profile quick --duration 30s

# Profile analysis
analyze-cpu: profile-cpu ## Analyze CPU profile in browser
	@echo "🌐 Opening CPU profile in browser..."
	go tool pprof -http=:8080 cpu.prof

analyze-memory: profile-memory ## Analyze memory profile in browser  
	@echo "🌐 Opening memory profile in browser..."
	go tool pprof -http=:8080 memory.prof

analyze-ensemble: profile-ensemble ## Analyze ensemble profile
	@echo "🌐 Opening ensemble profile in browser..."
	go tool pprof -http=:8080 ensemble.prof

# Compare before/after optimization
profile-baseline: ## Create baseline performance profile
	@echo "📊 Creating baseline profile..."
	@mkdir -p ./profiles/baseline
	go run main.go profile quick --duration 60s
	mv quick_profile.prof ./profiles/baseline/
	@echo "📄 Baseline saved to ./profiles/baseline/"

profile-compare: ## Compare current performance with baseline
	@echo "⚔️  Comparing performance..."
	@mkdir -p ./profiles/current
	go run main.go profile quick --duration 60s  
	mv quick_profile.prof ./profiles/current/
	@echo "📊 Compare with: go tool pprof -diff_base ./profiles/baseline/quick_profile.prof ./profiles/current/quick_profile.prof"

# All-in-one profiling
profile-all: ## Run comprehensive profiling suite
	@echo "🔬 Comprehensive profiling..."
	@make profile-cpu
	@make profile-memory  
	@make profile-ensemble
	@echo "✅ All profiles complete! Use 'make analyze-*' to view results"

# Enhanced ensemble profiling with network timing
profile-ensemble-detailed: ## Detailed ensemble profiling with network breakdown
	@echo "🎭 Detailed ensemble profiling..."
	go run main.go profile ensemble \
		--file $(OUTPUT_DIR)/eval_balanced.csv \
		--cases 20 \
		--output ensemble_detailed.prof \
		--verbose \
		--trace

# Compare network vs CPU performance
profile-network-analysis: ## Analyze network vs CPU performance
	@echo "🌐 Network performance analysis..."
	@echo "1️⃣  CPU-only profiling..."
	@make profile-cpu PROFILE_ITERATIONS=20
	@echo "\n2️⃣  Ensemble with network..."
	@make profile-ensemble-detailed
	@echo "\n📊 Compare with:"
	@echo "   CPU only: cpu.prof"
	@echo "   Full ensemble: ensemble_detailed.prof"

# Real-time latency monitoring
profile-realtime: ## Real-time latency monitoring
	@echo "📡 Real-time ensemble monitoring..."
	go run main.go profile ensemble \
		--file $(OUTPUT_DIR)/eval_balanced.csv \
		--cases 50 \
		--verbose | tee ensemble_realtime.log

# Blocking profile (network I/O waiting time)
profile-blocking: ## Profile blocking operations (network, I/O waits)
	@echo "🚧 Blocking operations profiling..."
	go run main.go profile blocking --input "git st" --iterations 50 --output blocking.prof

# Goroutine profile
profile-goroutine: ## Profile goroutine usage and patterns
	@echo "🔀 Goroutine profiling..."
	go run main.go profile goroutine --input "git st" --iterations 50 --output goroutine.prof

# All profile types at once
profile-comprehensive: ## Run all profile types (CPU, memory, blocking, goroutine, mutex)
	@echo "🔬 Comprehensive profiling..."
	go run main.go profile all-types --input "git st" --iterations 30 --output comprehensive.prof

# Analyze blocking profile (key for network timing!)
analyze-blocking: profile-blocking ## Analyze blocking profile for network waits
	@echo "🌐 Opening blocking profile (shows network waits)..."
	go tool pprof -http=:8080 blocking_blocking.prof

# Compare all profiles
analyze-all: profile-comprehensive ## Open all profiles in different ports
	@echo "🔍 Opening all profiles..."
	@echo "CPU (compute):     http://localhost:8080"
	@echo "Blocking (I/O):    http://localhost:8081" 
	@echo "Goroutines:        http://localhost:8082"
	@echo "Memory:            http://localhost:8083"
	go tool pprof -http=:8080 comprehensive_cpu.prof &
	go tool pprof -http=:8081 comprehensive_blocking.prof &
	go tool pprof -http=:8082 comprehensive_goroutine.prof &
	go tool pprof -http=:8083 comprehensive_memory.prof &
	@echo "🎯 Focus on BLOCKING profile for network timing!"

# Quick network timing analysis
profile-network-wait: ## Quick analysis of network wait times
	@echo "⚡ Quick network wait analysis..."
	go run main.go profile blocking --input "git st" --iterations 20
	@echo "\n🔍 Check blocking profile for network waits:"
	@echo "   go tool pprof -top comprehensive_blocking.prof"

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
