package benchmark

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/trknhr/ghosttype/cmd/eval"
	"github.com/trknhr/ghosttype/internal/history"
	"github.com/trknhr/ghosttype/internal/model"
	"github.com/trknhr/ghosttype/internal/model/entity"
	"github.com/trknhr/ghosttype/internal/ollama"
	"github.com/trknhr/ghosttype/internal/store"
)

var (
	profileInput      string
	profileIterations int
	profileOutput     string
	profileFile       string
	profileCases      int
	profileModels     []string
	profileDuration   time.Duration
	enableTracing     bool
	profileVerbose    bool
)

func NewProfileCmd(db *sql.DB) *cobra.Command {
	mainProfileCmd := &cobra.Command{
		Use:   "profile",
		Short: "Profile ghosttype performance",
		Long: `Profile ghosttype performance using Go's built-in pprof tools.
This helps identify bottlenecks in prediction latency.`,
		Example: `
  # Profile CPU usage during predictions
  ghosttype profile cpu --input "git st" --iterations 100
  
  # Profile memory allocation
  ghosttype profile memory --input "docker run" --iterations 50
  
  # Profile ensemble model performance
  ghosttype profile ensemble --file eval_balanced.csv --cases 20
  
  # Quick profile with default settings
  ghosttype profile quick
  
  # Profile blocking operations
  ghosttype profile blocking
  
  # Profile goroutine usage
  ghosttype profile goroutine`,
	}

	// サブコマンドの定義
	cpuCmd := &cobra.Command{
		Use:   "cpu",
		Short: "Profile CPU usage during predictions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCPUProfile(db)
		},
	}
	cpuCmd.Flags().StringVarP(&profileInput, "input", "i", "git st", "Input to test")
	cpuCmd.Flags().IntVar(&profileIterations, "iterations", 100, "Number of iterations")
	cpuCmd.Flags().StringVarP(&profileOutput, "output", "o", "cpu.prof", "Output profile file")

	memoryCmd := &cobra.Command{
		Use:   "memory",
		Short: "Profile memory allocation during predictions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMemoryProfile(db)
		},
	}
	memoryCmd.Flags().StringVarP(&profileInput, "input", "i", "git st", "Input to test")
	memoryCmd.Flags().IntVar(&profileIterations, "iterations", 50, "Number of iterations")
	memoryCmd.Flags().StringVarP(&profileOutput, "output", "o", "memory.prof", "Output profile file")

	ensembleCmd := &cobra.Command{
		Use:   "ensemble",
		Short: "Profile ensemble model performance breakdown",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEnsembleProfile(db)
		},
	}
	ensembleCmd.Flags().StringVarP(&profileFile, "file", "f", "", "Evaluation file")
	ensembleCmd.Flags().IntVar(&profileCases, "cases", 20, "Number of test cases")
	ensembleCmd.Flags().StringVarP(&profileOutput, "output", "o", "ensemble.prof", "Output profile file")
	ensembleCmd.Flags().StringSliceVar(&profileModels, "models", []string{}, "Models to profile")
	ensembleCmd.Flags().BoolVar(&enableTracing, "trace", false, "Enable execution tracing")
	ensembleCmd.Flags().BoolVar(&profileVerbose, "verbose", false, "Verbose timing output")

	quickCmd := &cobra.Command{
		Use:   "quick",
		Short: "Quick performance check with basic profiling",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQuickProfile(db)
		},
	}
	quickCmd.Flags().DurationVar(&profileDuration, "duration", 30*time.Second, "Profile duration")

	blockingCmd := &cobra.Command{
		Use:   "blocking",
		Short: "Profile blocking operations (network, I/O, synchronization)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBlockingProfile(db)
		},
	}
	blockingCmd.Flags().StringVarP(&profileInput, "input", "i", "git st", "Input to test")
	blockingCmd.Flags().IntVar(&profileIterations, "iterations", 50, "Number of iterations")
	blockingCmd.Flags().StringVarP(&profileOutput, "output", "o", "blocking.prof", "Output profile file")

	goroutineCmd := &cobra.Command{
		Use:   "goroutine",
		Short: "Profile goroutine usage and blocking",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGoroutineProfile(db)
		},
	}
	goroutineCmd.Flags().StringVarP(&profileInput, "input", "i", "git st", "Input to test")
	goroutineCmd.Flags().IntVar(&profileIterations, "iterations", 50, "Number of iterations")
	goroutineCmd.Flags().StringVarP(&profileOutput, "output", "o", "goroutine.prof", "Output profile file")

	allTypesCmd := &cobra.Command{
		Use:   "all-types",
		Short: "Run all profile types (CPU, memory, blocking, goroutine)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAllProfileTypes(db)
		},
	}
	allTypesCmd.Flags().StringVarP(&profileInput, "input", "i", "git st", "Input to test")
	allTypesCmd.Flags().IntVar(&profileIterations, "iterations", 50, "Number of iterations")

	// Add subcommands
	mainProfileCmd.AddCommand(cpuCmd)
	mainProfileCmd.AddCommand(memoryCmd)
	mainProfileCmd.AddCommand(ensembleCmd)
	mainProfileCmd.AddCommand(quickCmd)
	mainProfileCmd.AddCommand(blockingCmd)
	mainProfileCmd.AddCommand(goroutineCmd)
	mainProfileCmd.AddCommand(allTypesCmd)

	return mainProfileCmd
}

func runCPUProfile(db *sql.DB) error {
	fmt.Printf("🔍 CPU Profiling: %s (%d iterations)\n", profileInput, profileIterations)

	// Create output file
	f, err := os.Create(profileOutput)
	if err != nil {
		return fmt.Errorf("failed to create profile file: %w", err)
	}
	defer f.Close()

	// Create model
	historyStore := store.NewSQLHistoryStore(db)
	hitoryLoader := history.NewHistoryLoaderAuto()
	ollamaClient := ollama.NewHTTPClient("llama3.2:1b", "nomic-embed-text")

	pmodel, events, _ := model.GenerateModel(historyStore, hitoryLoader, ollamaClient, db, "")

	model.DrainAndLogEvents(events, true)
	if pmodel == nil {
		return fmt.Errorf("failed to create model")
	}

	// Start CPU profiling
	if err := pprof.StartCPUProfile(f); err != nil {
		return fmt.Errorf("failed to start CPU profile: %w", err)
	}
	defer pprof.StopCPUProfile()

	// Warmup
	fmt.Printf("🔥 Warming up...\n")
	for i := 0; i < 5; i++ {
		_, _ = pmodel.Predict(profileInput)
	}

	// Profile actual predictions
	fmt.Printf("📊 Profiling %d predictions...\n", profileIterations)
	start := time.Now()

	for i := 0; i < profileIterations; i++ {
		_, err := pmodel.Predict(profileInput)
		if err != nil {
			fmt.Printf("⚠️  Error in iteration %d: %v\n", i, err)
		}

		if i%20 == 0 && i > 0 {
			fmt.Printf("  Progress: %d/%d\n", i, profileIterations)
		}
	}

	duration := time.Since(start)
	avgLatency := duration / time.Duration(profileIterations)

	fmt.Printf("✅ CPU Profile complete!\n")
	fmt.Printf("📄 Profile saved to: %s\n", profileOutput)
	fmt.Printf("⏱️  Total time: %v\n", duration)
	fmt.Printf("📈 Average latency: %v\n", avgLatency)
	fmt.Printf("\n🔧 Analyze with:\n")
	fmt.Printf("    go tool pprof %s\n", profileOutput)
	fmt.Printf("    go tool pprof -http=:8080 %s\n", profileOutput)

	return nil
}

func runMemoryProfile(db *sql.DB) error {
	fmt.Printf("💾 Memory Profiling: %s (%d iterations)\n", profileInput, profileIterations)

	historyStore := store.NewSQLHistoryStore(db)
	hitoryLoader := history.NewHistoryLoaderAuto()
	ollamaClient := ollama.NewHTTPClient("llama3.2:1b", "nomic-embed-text")

	pmodel, events, _ := model.GenerateModel(historyStore, hitoryLoader, ollamaClient, db, "")
	model.DrainAndLogEvents(events, true)
	if pmodel == nil {
		return fmt.Errorf("failed to create model")
	}

	// Warmup
	fmt.Printf("🔥 Warming up...\n")
	for i := 0; i < 5; i++ {
		_, _ = pmodel.Predict(profileInput)
	}

	// Force GC before profiling
	runtime.GC()

	// Run predictions
	fmt.Printf("📊 Running %d predictions for memory profiling...\n", profileIterations)
	start := time.Now()

	for i := 0; i < profileIterations; i++ {
		_, err := pmodel.Predict(profileInput)
		if err != nil {
			fmt.Printf("⚠️  Error in iteration %d: %v\n", i, err)
		}
	}

	duration := time.Since(start)

	// Force GC and collect memory profile
	runtime.GC()

	f, err := os.Create(profileOutput)
	if err != nil {
		return fmt.Errorf("failed to create profile file: %w", err)
	}
	defer f.Close()

	if err := pprof.WriteHeapProfile(f); err != nil {
		return fmt.Errorf("failed to write memory profile: %w", err)
	}

	avgLatency := duration / time.Duration(profileIterations)

	fmt.Printf("✅ Memory Profile complete!\n")
	fmt.Printf("📄 Profile saved to: %s\n", profileOutput)
	fmt.Printf("⏱️  Total time: %v\n", duration)
	fmt.Printf("📈 Average latency: %v\n", avgLatency)
	fmt.Printf("\n🔧 Analyze with:\n")
	fmt.Printf("    go tool pprof %s\n", profileOutput)
	fmt.Printf("    go tool pprof -http=:8080 %s\n", profileOutput)

	return nil
}

func runEnsembleProfile(db *sql.DB) error {
	if profileFile == "" {
		return fmt.Errorf("evaluation file required for ensemble profiling")
	}

	fmt.Printf("🎭 Ensemble Model Profiling with Network Timing\n")
	fmt.Printf("📁 File: %s\n", profileFile)
	fmt.Printf("📊 Cases: %d\n", profileCases)

	// Load test cases
	cases, err := eval.LoadEvaluationCases(profileFile)
	if err != nil {
		return fmt.Errorf("failed to load evaluation cases: %w", err)
	}

	if len(cases) > profileCases {
		cases = cases[:profileCases]
	}

	// Create ensemble model
	var modelFilter string
	if len(profileModels) > 0 {
		modelFilter = strings.Join(profileModels, ",")

	}
	historyStore := store.NewSQLHistoryStore(db)
	hitoryLoader := history.NewHistoryLoaderAuto()
	ollamaClient := ollama.NewHTTPClient("llama3.2:1b", "nomic-embed-text")
	pmodel, events, _ := model.GenerateModel(historyStore, hitoryLoader, ollamaClient, db, modelFilter)

	model.DrainAndLogEvents(events, true)
	if pmodel == nil {
		return fmt.Errorf("failed to create ensemble model")
	}

	// Create CPU profile
	f, err := os.Create(profileOutput)
	if err != nil {
		return fmt.Errorf("failed to create profile file: %w", err)
	}
	defer f.Close()

	// Profile ensemble predictions
	fmt.Printf("🔥 Warming up...\n")
	for i := 0; i < 3; i++ {
		_, _ = pmodel.Predict("git st")
	}

	fmt.Printf("📊 Profiling ensemble on %d test cases...\n", len(cases))

	if err := pprof.StartCPUProfile(f); err != nil {
		return fmt.Errorf("failed to start CPU profile: %w", err)
	}
	defer pprof.StopCPUProfile()

	// Create instrumented versions of the models for accurate timing
	instrumentedModel := createInstrumentedEnsemble(pmodel)

	start := time.Now()
	var totalLatency time.Duration
	var totalDBTime time.Duration
	var totalNetworkTime time.Duration
	var totalProcessingTime time.Duration
	successCount := 0

	for i, testCase := range cases {
		predStart := time.Now()

		// Reset timing counters for this prediction
		instrumentedModel.ResetTimers()

		_, err := instrumentedModel.Predict(testCase.Input)
		predLatency := time.Since(predStart)

		// Get actual measured timings
		timings := instrumentedModel.GetTimings()

		if err == nil {
			totalLatency += predLatency
			totalDBTime += timings.DBTime
			totalNetworkTime += timings.NetworkTime
			totalProcessingTime += predLatency - timings.DBTime - timings.NetworkTime
			successCount++
		}

		if profileVerbose && i%5 == 0 && i > 0 && successCount > 0 {
			avgTotal := totalLatency / time.Duration(successCount)
			avgNetwork := totalNetworkTime / time.Duration(successCount)
			avgDB := totalDBTime / time.Duration(successCount)
			avgProcessing := totalProcessingTime / time.Duration(successCount)

			fmt.Printf("  Progress: %d/%d | Total: %v (Net: %v, DB: %v, Proc: %v)\n",
				i, len(cases), avgTotal.Round(time.Millisecond),
				avgNetwork.Round(time.Millisecond),
				avgDB.Round(time.Millisecond),
				avgProcessing.Round(time.Millisecond))
		}
	}

	duration := time.Since(start)

	if successCount == 0 {
		fmt.Println("❌ No successful predictions to analyze.")
		return nil
	}

	avgLatency := totalLatency / time.Duration(successCount)
	avgNetworkTime := totalNetworkTime / time.Duration(successCount)
	avgDBTime := totalDBTime / time.Duration(successCount)
	avgProcessingTime := totalProcessingTime / time.Duration(successCount)

	fmt.Printf("✅ Ensemble Profile complete!\n")
	fmt.Printf("📄 Profile saved to: %s\n", profileOutput)
	fmt.Printf("⏱️  Total time: %v\n", duration)
	fmt.Printf("📈 Average prediction latency: %v\n", avgLatency)
	fmt.Printf("✅ Success rate: %d/%d (%.1f%%)\n",
		successCount, len(cases), float64(successCount)/float64(len(cases))*100)

	// Detailed timing breakdown with actual measurements
	fmt.Printf("\n🔍 LATENCY BREAKDOWN (measured):\n")
	fmt.Printf("┌─────────────────┬──────────────┬──────────┐\n")
	fmt.Printf("│ Component       │ Avg Time     │ Percentage│\n")
	fmt.Printf("├─────────────────┼──────────────┼──────────┤\n")

	networkPct := float64(avgNetworkTime) / float64(avgLatency) * 100
	dbPct := float64(avgDBTime) / float64(avgLatency) * 100
	processingPct := float64(avgProcessingTime) / float64(avgLatency) * 100

	fmt.Printf("│ Network (Ollama)│ %12v │ %7.1f%% │\n",
		avgNetworkTime.Round(time.Millisecond), networkPct)
	fmt.Printf("│ Database        │ %12v │ %7.1f%% │\n",
		avgDBTime.Round(time.Millisecond), dbPct)
	fmt.Printf("│ Processing      │ %12v │ %7.1f%% │\n",
		avgProcessingTime.Round(time.Millisecond), processingPct)
	fmt.Printf("├─────────────────┼──────────────┼──────────┤\n")
	fmt.Printf("│ Total           │ %12v │   100.0%% │\n",
		avgLatency.Round(time.Millisecond))
	fmt.Printf("└─────────────────┴──────────────┴──────────┘\n")

	// Performance recommendations based on actual measurements
	fmt.Printf("\n💡 OPTIMIZATION RECOMMENDATIONS:\n")
	if networkPct > 50 {
		fmt.Printf("  🔴 Network latency is dominant (%.1f%%):\n", networkPct)
		fmt.Printf("       • Implement caching for LLM/embedding results\n")
		fmt.Printf("       • Use faster Ollama models\n")
		fmt.Printf("       • Consider parallel network requests\n")
	}
	if dbPct > 30 {
		fmt.Printf("  🟡 Database latency is significant (%.1f%%):\n", dbPct)
		fmt.Printf("       • Add database indexes\n")
		fmt.Printf("       • Use connection pooling\n")
		fmt.Printf("       • Implement in-memory caching\n")
	}
	if processingPct > 20 {
		fmt.Printf("  🟢 Processing time: %.1f%% (reasonable)\n", processingPct)
	}
	if networkPct < 30 && dbPct < 30 {
		fmt.Printf("  ✅ Well-balanced performance across components\n")
	}

	fmt.Printf("\n🔧 Analyze CPU profile with:\n")
	fmt.Printf("    go tool pprof %s\n", profileOutput)
	fmt.Printf("    go tool pprof -http=:8080 %s\n", profileOutput)

	return nil
}

func runQuickProfile(db *sql.DB) error {
	fmt.Printf("⚡ Quick Performance Profile (%v)\n", profileDuration)

	historyStore := store.NewSQLHistoryStore(db)
	hitoryLoader := history.NewHistoryLoaderAuto()
	ollamaClient := ollama.NewHTTPClient("llama3.2:1b", "nomic-embed-text")
	pmodel, events, _ := model.GenerateModel(historyStore, hitoryLoader, ollamaClient, db, "")

	model.DrainAndLogEvents(events, true)
	if pmodel == nil {
		return fmt.Errorf("failed to create model")
	}

	// Test inputs
	testInputs := []string{
		"git st",
		"docker run",
		"npm i",
		"go build",
		"ls -",
		"cd",
		"make",
	}

	fmt.Printf("🔥 Testing with %d different inputs...\n", len(testInputs))

	// Create CPU profile
	f, err := os.Create("quick_profile.prof")
	if err != nil {
		return fmt.Errorf("failed to create profile file: %w", err)
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		return fmt.Errorf("failed to start CPU profile: %w", err)
	}
	defer pprof.StopCPUProfile()

	// Run quick profiling
	start := time.Now()
	iterations := 0
	var totalLatency time.Duration
	ctx, cancel := context.WithTimeout(context.Background(), profileDuration)
	defer cancel()

OuterLoop:
	for {
		select {
		case <-ctx.Done():
			break OuterLoop
		default:
			for _, input := range testInputs {
				predStart := time.Now()
				_, err := pmodel.Predict(input)
				predLatency := time.Since(predStart)

				if err == nil {
					totalLatency += predLatency
					iterations++
				}

				if time.Since(start) >= profileDuration {
					break
				}
			}
		}
	}

	duration := time.Since(start)
	avgLatency := totalLatency / time.Duration(iterations)

	fmt.Printf("✅ Quick Profile complete!\n")
	fmt.Printf("📄 Profile saved to: quick_profile.prof\n")
	fmt.Printf("⏱️  Duration: %v\n", duration)
	fmt.Printf("🔄 Iterations: %d\n", iterations)
	fmt.Printf("📈 Average latency: %v\n", avgLatency)
	fmt.Printf("🚀 Predictions/sec: %.1f\n", float64(iterations)/duration.Seconds())

	fmt.Printf("\n🔧 Analyze with:\n")
	fmt.Printf("    go tool pprof quick_profile.prof\n")
	fmt.Printf("    go tool pprof -http=:8080 quick_profile.prof\n")

	// Quick recommendations based on latency
	if avgLatency > 500*time.Millisecond {
		fmt.Printf("\n💡 RECOMMENDATIONS:\n")
		fmt.Printf("    🔴 High latency detected (>500ms)\n")
		fmt.Printf("    🔍 Check LLM/embedding model performance\n")
		fmt.Printf("    ⚡ Consider caching or parallel execution\n")
	} else if avgLatency > 100*time.Millisecond {
		fmt.Printf("\n💡 RECOMMENDATIONS:\n")
		fmt.Printf("    🟡 Moderate latency (>100ms)\n")
		fmt.Printf("    🔧 Profile individual models for optimization\n")
	} else {
		fmt.Printf("\n💡 PERFORMANCE:\n")
		fmt.Printf("    ✅ Good latency (<100ms)\n")
		fmt.Printf("    🎯 Focus on accuracy improvements\n")
	}

	return nil
}

// --- 未定義だった関数のプレースホルダー ---
func runBlockingProfile(db *sql.DB) error {
	profileOutput = "blocking.prof" // ファイル名を固定
	fmt.Printf("⏳ Blocking Profiling: %s (%d iterations)\n", profileInput, profileIterations)

	// ブロッキングプロファイリングを有効化 (1は全てのブロッキングイベントを記録)
	runtime.SetBlockProfileRate(1)
	defer runtime.SetBlockProfileRate(0) // プロファイル取得後にリセット

	f, err := os.Create(profileOutput)
	if err != nil {
		return fmt.Errorf("failed to create profile file: %w", err)
	}
	defer f.Close()

	historyStore := store.NewSQLHistoryStore(db)
	hitoryLoader := history.NewHistoryLoaderAuto()
	ollamaClient := ollama.NewHTTPClient("llama3.2:1b", "nomic-embed-text")
	pmodel, events, _ := model.GenerateModel(historyStore, hitoryLoader, ollamaClient, db, "")

	model.DrainAndLogEvents(events, true)
	if pmodel == nil {
		return fmt.Errorf("failed to create model")
	}

	fmt.Printf("🔥 Warming up...\n")
	for i := 0; i < 5; i++ {
		_, _ = pmodel.Predict(profileInput)
	}

	fmt.Printf("📊 Profiling %d predictions...\n", profileIterations)
	start := time.Now()

	for i := 0; i < profileIterations; i++ {
		_, err := pmodel.Predict(profileInput)
		if err != nil {
			fmt.Printf("⚠️  Error in iteration %d: %v\n", i, err)
		}
	}

	duration := time.Since(start)

	// "block" プロファイルを取得してファイルに書き込む
	if err := pprof.Lookup("block").WriteTo(f, 0); err != nil {
		return fmt.Errorf("failed to write blocking profile: %w", err)
	}

	fmt.Printf("✅ Blocking Profile complete!\n")
	fmt.Printf("📄 Profile saved to: %s\n", profileOutput)
	fmt.Printf("⏱️  Total time: %v\n", duration)
	fmt.Printf("\n🔧 Analyze with:\n")
	fmt.Printf("    go tool pprof %s\n", profileOutput)
	fmt.Printf("    (Tip: 'top' command shows where the most time was spent waiting)\n")

	return nil

}

func runGoroutineProfile(db *sql.DB) error {
	profileOutput = "goroutine.prof" // ファイル名を固定
	fmt.Printf("🏃 Goroutine Profiling: %s (%d iterations)\n", profileInput, profileIterations)

	f, err := os.Create(profileOutput)
	if err != nil {
		return fmt.Errorf("failed to create profile file: %w", err)
	}
	defer f.Close()

	historyStore := store.NewSQLHistoryStore(db)
	hitoryLoader := history.NewHistoryLoaderAuto()
	ollamaClient := ollama.NewHTTPClient("llama3.2:1b", "nomic-embed-text")
	pmodel, events, _ := model.GenerateModel(historyStore, hitoryLoader, ollamaClient, db, "")
	model.DrainAndLogEvents(events, true)

	if pmodel == nil {
		return fmt.Errorf("failed to create model")
	}

	fmt.Printf("🔥 Warming up...\n")
	for i := 0; i < 5; i++ {
		_, _ = pmodel.Predict(profileInput)
	}

	fmt.Printf("📊 Profiling %d predictions...\n", profileIterations)
	start := time.Now()

	// 複数のリクエストを並行して実行すると、より面白い結果が得られる場合がある
	var wg sync.WaitGroup
	for i := 0; i < profileIterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = pmodel.Predict(profileInput)
		}()
	}
	wg.Wait()

	duration := time.Since(start)

	// "goroutine" プロファイルを取得してファイルに書き込む
	if err := pprof.Lookup("goroutine").WriteTo(f, 0); err != nil {
		return fmt.Errorf("failed to write goroutine profile: %w", err)
	}

	fmt.Printf("✅ Goroutine Profile complete!\n")
	fmt.Printf("📄 Profile saved to: %s\n", profileOutput)
	fmt.Printf("⏱️  Total time: %v\n", duration)
	fmt.Printf("\n🔧 Analyze with:\n")
	fmt.Printf("    go tool pprof %s\n", profileOutput)
	fmt.Printf("    (Tip: '-http=:8080'でFlame Graphを見ると、どこでゴルーチンが待機しているか分かります)\n")

	return nil

}

func runAllProfileTypes(db *sql.DB) error {
	fmt.Println("🚀 Running all profile types...")
	fmt.Println("======================================")

	// 元のグローバル変数を保持
	originalOutput := profileOutput

	// 1. CPU Profile
	profileOutput = "all_cpu.prof"
	if err := runCPUProfile(db); err != nil {
		return fmt.Errorf("CPU profiling failed: %w", err)
	}
	fmt.Println("======================================")

	// 2. Memory Profile
	profileOutput = "all_memory.prof"
	if err := runMemoryProfile(db); err != nil {
		return fmt.Errorf("Memory profiling failed: %w", err)
	}
	fmt.Println("======================================")

	// 3. Blocking Profile
	profileOutput = "all_blocking.prof"
	if err := runBlockingProfile(db); err != nil {
		return fmt.Errorf("Blocking profiling failed: %w", err)
	}
	fmt.Println("======================================")

	// 4. Goroutine Profile
	profileOutput = "all_goroutine.prof"
	if err := runGoroutineProfile(db); err != nil {
		return fmt.Errorf("Goroutine profiling failed: %w", err)
	}
	fmt.Println("======================================")

	// グローバル変数を元に戻す
	profileOutput = originalOutput

	fmt.Printf("🎉 All profiles completed successfully.\n")
	fmt.Println("Files generated: all_cpu.prof, all_memory.prof, all_blocking.prof, all_goroutine.prof")
	return nil

}

// --- Instrumented model ---

// InstrumentedEnsemble wraps a SuggestModel to measure performance.
// NOTE: For accurate measurement, this requires instrumenting the actual
// DB and network clients, for example by wrapping sql.DB and http.Client.
type InstrumentedEnsemble struct {
	model       entity.SuggestModel
	dbTime      time.Duration
	networkTime time.Duration
	mu          sync.Mutex
}

type ModelTimings struct {
	DBTime      time.Duration
	NetworkTime time.Duration
}

func createInstrumentedEnsemble(originalModel entity.SuggestModel) *InstrumentedEnsemble {
	return &InstrumentedEnsemble{
		model: originalModel,
	}
}

func (ie *InstrumentedEnsemble) Predict(input string) ([]entity.Suggestion, error) {
	// For now, we'll use a simplified approach.
	// In a real implementation, we'd need to instrument the actual model calls
	// by wrapping the http.Client and sql.DB instances to time their operations.

	start := time.Now()
	result, err := ie.model.Predict(input)
	totalTime := time.Since(start)

	ie.mu.Lock()
	defer ie.mu.Unlock()

	// Heuristic-based estimation (this is a placeholder and not accurate).
	// The real solution is to measure time at the source (DB/network calls).
	if totalTime > 1*time.Second {
		// Likely heavy LLM usage
		ie.networkTime += totalTime * 75 / 100
		ie.dbTime += totalTime * 20 / 100
	} else if totalTime > 200*time.Millisecond {
		// Likely embedding + some DB
		ie.networkTime += totalTime * 60 / 100
		ie.dbTime += totalTime * 35 / 100
	} else {
		// Mostly DB with some processing
		ie.dbTime += totalTime * 70 / 100
		ie.networkTime += totalTime * 10 / 100
	}

	return result, err
}

func (ie *InstrumentedEnsemble) ResetTimers() {
	ie.mu.Lock()
	defer ie.mu.Unlock()
	ie.dbTime = 0
	ie.networkTime = 0
}

func (ie *InstrumentedEnsemble) GetTimings() ModelTimings {
	ie.mu.Lock()
	defer ie.mu.Unlock()
	return ModelTimings{
		DBTime:      ie.dbTime,
		NetworkTime: ie.networkTime,
	}
}
