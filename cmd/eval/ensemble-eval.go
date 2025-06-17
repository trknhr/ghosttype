package eval

import (
	"bufio"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/trknhr/ghosttype/internal/history"
	"github.com/trknhr/ghosttype/internal/model"
	"github.com/trknhr/ghosttype/internal/model/entity"
	"github.com/trknhr/ghosttype/internal/ollama"
	"github.com/trknhr/ghosttype/internal/store"
)

type EvaluationCase struct {
	Input    string `json:"input"`
	Expected string `json:"expected"`
	Category string `json:"category,omitempty"`
	Source   string `json:"source,omitempty"` // Optional source field for additional context
}

type EvaluationResult struct {
	Total       int
	Top1Correct int
	Top3Correct int
	ByCategory  map[string]CategoryResult
}

type CategoryResult struct {
	Total       int
	Top1Correct int
	Top3Correct int
}

type EnsembleEvaluationResult struct {
	Total        int
	Top1Correct  int
	Top3Correct  int
	Top5Correct  int
	Top10Correct int
	ByCategory   map[string]EnsembleCategoryResult
	ModelContrib map[string]ModelContribution // Each model's contribution
	AvgRank      float64                      // Average rank of correct answer
	MRR          float64                      // Mean Reciprocal Rank
}

type EnsembleCategoryResult struct {
	Total        int
	Top1Correct  int
	Top3Correct  int
	Top5Correct  int
	Top10Correct int
	AvgRank      float64
}

type ModelContribution struct {
	ModelName    string
	HitsProvided int     // How many times this model provided the correct answer
	Weight       float64 // Model's weight in ensemble
	AvgScore     float64 // Average score this model gives
	UniqueHits   int     // Hits only this model found
}

type RankPosition struct {
	Found bool
	Rank  int
}

func NewEnsembleEvalCmd(db *sql.DB) *cobra.Command {
	var evalFile string
	var modelNames []string
	var includeIndividual bool
	var maxSuggestions int

	cmd := &cobra.Command{
		Use:   "ensemble-eval",
		Short: "Evaluate ensemble model (mimics production behavior)",
		Long: `Evaluate the ensemble model exactly as it works in production.
This combines multiple models with their weights and timeouts, providing
a realistic assessment of the actual user experience.`,
		Example: `
  # Evaluate ensemble with default models
  ghosttype ensemble-eval -f eval_balanced.csv
  
  # Evaluate specific model combination
  ghosttype ensemble-eval -f eval_balanced.csv -m freq,embedding,llm
  
  # Include individual model breakdowns
  ghosttype ensemble-eval -f eval_balanced.csv --include-individual`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunEnsembleEvaluation(db, evalFile, modelNames, includeIndividual, maxSuggestions)
		},
	}

	cmd.Flags().StringVarP(&evalFile, "file", "f", "", "Path to CSV or JSONL evaluation file")
	cmd.Flags().StringSliceVarP(&modelNames, "models", "m", []string{}, "Models to include (empty = all enabled)")
	cmd.Flags().BoolVar(&includeIndividual, "include-individual", false, "Show individual model performance")
	cmd.Flags().IntVar(&maxSuggestions, "max-suggestions", 20, "Maximum suggestions to evaluate")
	cmd.MarkFlagRequired("file")

	return cmd
}

func RunEnsembleEvaluation(db *sql.DB, filePath string, modelNames []string, includeIndividual bool, maxSuggestions int) error {
	// Load test cases
	cases, err := LoadEvaluationCases(filePath)
	if err != nil {
		return fmt.Errorf("failed to load evaluation cases: %w", err)
	}

	fmt.Printf("🎯 ENSEMBLE MODEL EVALUATION\n")
	fmt.Printf("═══════════════════════════════════════\n")
	fmt.Printf("📊 Test cases: %d\n", len(cases))
	fmt.Printf("🔧 Max suggestions: %d\n", maxSuggestions)

	// Create ensemble model (same as production)
	var filterModels string
	if len(modelNames) > 0 {
		filterModels = strings.Join(modelNames, ",")
	}

	historyStore := store.NewSQLHistoryStore(db)
	hitoryLoader := history.NewHistoryLoaderAuto()
	ollamaClient := ollama.NewHTTPClient("llama3.2:1b", "nomic-embed-text")
	ensembleModel, events, _ := model.GenerateModel(historyStore, hitoryLoader, ollamaClient, db, filterModels)
	model.DrainAndLogEvents(events, true)
	if ensembleModel == nil {
		return fmt.Errorf("failed to create ensemble model")
	}

	fmt.Printf("🚀 Model created, starting evaluation...\n")

	// Initialize results
	result := EnsembleEvaluationResult{
		ByCategory:   make(map[string]EnsembleCategoryResult),
		ModelContrib: make(map[string]ModelContribution),
	}

	var totalRank float64
	var totalReciprocalRank float64
	validRanks := 0

	start := time.Now()

	// Process each test case
	for i, testCase := range cases {
		if i%50 == 0 && i > 0 {
			fmt.Printf("  📈 Progress: %d/%d (%.1f%%)\n", i, len(cases), float64(i)/float64(len(cases))*100)
		}

		suggestions, err := ensembleModel.Predict(testCase.Input)
		if err != nil {
			fmt.Printf("⚠️  Prediction error for %q: %v\n", testCase.Input, err)
			continue
		}

		result.Total++

		// Initialize category if not exists
		if _, exists := result.ByCategory[testCase.Category]; !exists {
			result.ByCategory[testCase.Category] = EnsembleCategoryResult{}
		}
		categoryResult := result.ByCategory[testCase.Category]
		categoryResult.Total++

		// Find the rank of correct answer
		rankPos := findRankPosition(suggestions, testCase.Expected, maxSuggestions)

		if rankPos.Found {
			totalRank += float64(rankPos.Rank)
			totalReciprocalRank += 1.0 / float64(rankPos.Rank)
			validRanks++
			categoryResult.AvgRank += float64(rankPos.Rank)

			// Update accuracy metrics
			if rankPos.Rank == 1 {
				result.Top1Correct++
				categoryResult.Top1Correct++
			}
			if rankPos.Rank <= 3 {
				result.Top3Correct++
				categoryResult.Top3Correct++
			}
			if rankPos.Rank <= 5 {
				result.Top5Correct++
				categoryResult.Top5Correct++
			}
			if rankPos.Rank <= 10 {
				result.Top10Correct++
				categoryResult.Top10Correct++
			}
		} else {
			// Not found in top suggestions
			if includeIndividual {
				fmt.Printf("🔴 Miss | Input: %q | Expected: %q | Category: %s\n",
					testCase.Input, testCase.Expected, testCase.Category)
				printTopSuggestions(suggestions, 5)
			}
		}

		result.ByCategory[testCase.Category] = categoryResult
	}

	duration := time.Since(start)

	// Calculate final metrics
	if validRanks > 0 {
		result.AvgRank = totalRank / float64(validRanks)
		result.MRR = totalReciprocalRank / float64(validRanks)
	}

	// Finalize category averages
	for category, catResult := range result.ByCategory {
		if catResult.Top10Correct > 0 {
			catResult.AvgRank = catResult.AvgRank / float64(catResult.Top10Correct)
			result.ByCategory[category] = catResult
		}
	}

	// Print results
	printEnsembleResults(result, duration, includeIndividual)

	return nil
}

func findRankPosition(suggestions []entity.Suggestion, expected string, maxSuggestions int) RankPosition {
	limit := min(maxSuggestions, len(suggestions))

	for i := 0; i < limit; i++ {
		if suggestions[i].Text == expected {
			return RankPosition{Found: true, Rank: i + 1}
		}
	}

	return RankPosition{Found: false, Rank: -1}
}

func printEnsembleResults(result EnsembleEvaluationResult, duration time.Duration, showDetails bool) {
	fmt.Printf("\n🎯 ENSEMBLE EVALUATION RESULTS\n")
	fmt.Printf("═══════════════════════════════════════════════════════════\n")
	fmt.Printf("⏱️  Evaluation time: %v\n", duration.Round(time.Millisecond))
	fmt.Printf("📊 Total test cases: %d\n", result.Total)
	fmt.Printf("\n📈 ACCURACY METRICS:\n")

	// Main accuracy table
	fmt.Printf("┌─────────────┬──────────┬──────────┬─────────────┐\n")
	fmt.Printf("│ Metric      │ Count    │ Rate     │ Performance │\n")
	fmt.Printf("├─────────────┼──────────┼──────────┼─────────────┤\n")

	top1Rate := float64(result.Top1Correct) / float64(result.Total) * 100
	top3Rate := float64(result.Top3Correct) / float64(result.Total) * 100
	top5Rate := float64(result.Top5Correct) / float64(result.Total) * 100
	top10Rate := float64(result.Top10Correct) / float64(result.Total) * 100

	fmt.Printf("│ Top-1       │ %4d/%3d │ %6.2f%% │ %s │\n",
		result.Top1Correct, result.Total, top1Rate, getPerformanceIcon(top1Rate, 30, 20, 10))
	fmt.Printf("│ Top-3       │ %4d/%3d │ %6.2f%% │ %s │\n",
		result.Top3Correct, result.Total, top3Rate, getPerformanceIcon(top3Rate, 50, 35, 20))
	fmt.Printf("│ Top-5       │ %4d/%3d │ %6.2f%% │ %s │\n",
		result.Top5Correct, result.Total, top5Rate, getPerformanceIcon(top5Rate, 60, 45, 30))
	fmt.Printf("│ Top-10      │ %4d/%3d │ %6.2f%% │ %s │\n",
		result.Top10Correct, result.Total, top10Rate, getPerformanceIcon(top10Rate, 75, 60, 45))
	fmt.Printf("└─────────────┴──────────┴──────────┴─────────────┘\n")

	// Quality metrics
	fmt.Printf("\n📊 QUALITY METRICS:\n")
	fmt.Printf("  Average Rank: %.2f (lower is better)\n", result.AvgRank)
	fmt.Printf("  MRR (Mean Reciprocal Rank): %.3f (higher is better)\n", result.MRR)

	// Coverage analysis
	coverageRate := float64(result.Top10Correct) / float64(result.Total) * 100
	missRate := 100 - coverageRate
	fmt.Printf("  Coverage (Top-10): %.2f%%\n", coverageRate)
	fmt.Printf("  Miss Rate: %.2f%%\n", missRate)

	// Performance assessment
	fmt.Printf("\n🎯 OVERALL ASSESSMENT:\n")
	assessment := assessEnsemblePerformance(top1Rate, top10Rate, result.AvgRank, result.MRR)
	fmt.Printf("  %s\n", assessment)

	// Category breakdown
	if len(result.ByCategory) > 1 {
		fmt.Printf("\n📂 CATEGORY BREAKDOWN:\n")
		printCategoryTable(result.ByCategory)
	}

	// Recommendations
	fmt.Printf("\n💡 RECOMMENDATIONS:\n")
	printRecommendations(result)
}

func getPerformanceIcon(rate, excellent, good, fair float64) string {
	if rate >= excellent {
		return "🌟 Excellent"
	} else if rate >= good {
		return "✅ Good     "
	} else if rate >= fair {
		return "⚠️  Fair     "
	} else {
		return "❌ Poor     "
	}
}

func assessEnsemblePerformance(top1Rate, top10Rate, avgRank, mrr float64) string {
	if top1Rate >= 25 && top10Rate >= 70 && avgRank <= 3.0 {
		return "🌟 EXCELLENT: Production-ready performance. Users will find suggestions highly relevant."
	} else if top1Rate >= 15 && top10Rate >= 60 && avgRank <= 4.0 {
		return "✅ GOOD: Solid performance. Most users will find useful suggestions."
	} else if top1Rate >= 10 && top10Rate >= 45 && avgRank <= 6.0 {
		return "⚠️  FAIR: Acceptable but could be improved. Some users may find suggestions helpful."
	} else if top10Rate >= 30 {
		return "🔄 DEVELOPING: Basic functionality works, but needs significant improvement."
	} else {
		return "❌ POOR: Performance below acceptable threshold. Major improvements needed."
	}
}

func printCategoryTable(categories map[string]EnsembleCategoryResult) {
	// Sort categories by total cases
	type catPair struct {
		name   string
		result EnsembleCategoryResult
	}

	var sorted []catPair
	for name, result := range categories {
		sorted = append(sorted, catPair{name, result})
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].result.Total > sorted[j].result.Total
	})

	fmt.Printf("┌─────────────────┬───────┬────────┬────────┬────────┬─────────┐\n")
	fmt.Printf("│ Category        │ Cases │ Top-1  │ Top-3  │ Top-10 │ Avg Rank│\n")
	fmt.Printf("├─────────────────┼───────┼────────┼────────┼────────┼─────────┤\n")

	for _, cat := range sorted {
		name := cat.name
		result := cat.result

		if len(name) > 15 {
			name = name[:12] + "..."
		}

		top1Rate := float64(result.Top1Correct) / float64(result.Total) * 100
		top3Rate := float64(result.Top3Correct) / float64(result.Total) * 100
		top10Rate := float64(result.Top10Correct) / float64(result.Total) * 100

		fmt.Printf("│ %-15s │ %5d │ %5.1f%% │ %5.1f%% │ %5.1f%% │ %7.2f │\n",
			name, result.Total, top1Rate, top3Rate, top10Rate, result.AvgRank)
	}

	fmt.Printf("└─────────────────┴───────┴────────┴────────┴────────┴─────────┘\n")
}

func printRecommendations(result EnsembleEvaluationResult) {
	recommendations := []string{}

	top1Rate := float64(result.Top1Correct) / float64(result.Total) * 100
	top10Rate := float64(result.Top10Correct) / float64(result.Total) * 100

	if top1Rate < 15 {
		recommendations = append(recommendations,
			"• Improve Top-1 accuracy: Consider adjusting model weights or adding more training data")
	}

	if top10Rate < 60 {
		recommendations = append(recommendations,
			"• Increase coverage: Add more diverse models or improve fuzzy matching")
	}

	if result.AvgRank > 5.0 {
		recommendations = append(recommendations,
			"• Optimize ranking: Review scoring mechanisms and model weights")
	}

	if result.MRR < 0.3 {
		recommendations = append(recommendations,
			"• Enhance relevance: Focus on improving the quality of top suggestions")
	}

	// Category-specific recommendations
	for category, catResult := range result.ByCategory {
		categoryTop10 := float64(catResult.Top10Correct) / float64(catResult.Total) * 100
		if categoryTop10 < 40 && catResult.Total >= 10 {
			recommendations = append(recommendations,
				fmt.Sprintf("• Poor performance in '%s' category (%.1f%% Top-10): Review category-specific training",
					category, categoryTop10))
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "• Performance looks good! Consider A/B testing with users.")
	}

	for _, rec := range recommendations {
		fmt.Printf("  %s\n", rec)
	}
}

func printTopSuggestions(suggestions []entity.Suggestion, limit int) {
	limit = min(limit, len(suggestions))
	for i := 0; i < limit; i++ {
		fmt.Printf("    [%d] %s (%.3f)\n", i+1, suggestions[i].Text, suggestions[i].Score)
	}
}

func LoadEvaluationCases(filePath string) ([]EvaluationCase, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".csv":
		return loadFromCSV(filePath)
	case ".jsonl":
		return loadFromJSONL(filePath)
	default:
		return nil, fmt.Errorf("unsupported file format: %s (supported: .csv, .jsonl)", ext)
	}
}

func loadFromCSV(filePath string) ([]EvaluationCase, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("empty CSV file")
	}

	// Find column indices
	header := records[0]
	inputIdx, expectedIdx, categoryIdx := -1, -1, -1

	for i, col := range header {
		switch strings.ToLower(strings.TrimSpace(col)) {
		case "input":
			inputIdx = i
		case "expected":
			expectedIdx = i
		case "category":
			categoryIdx = i
		}
	}

	if inputIdx == -1 || expectedIdx == -1 {
		return nil, fmt.Errorf("CSV must contain 'input' and 'expected' columns")
	}

	var cases []EvaluationCase
	for i, record := range records[1:] { // Skip header
		if len(record) <= inputIdx || len(record) <= expectedIdx {
			fmt.Fprintf(os.Stderr, "⚠️  Skipping malformed row %d\n", i+2)
			continue
		}

		input := strings.TrimSpace(record[inputIdx])
		expected := strings.TrimSpace(record[expectedIdx])

		if input == "" || expected == "" {
			continue
		}

		category := "unknown"
		if categoryIdx != -1 && len(record) > categoryIdx {
			if cat := strings.TrimSpace(record[categoryIdx]); cat != "" {
				category = cat
			}
		}

		cases = append(cases, EvaluationCase{
			Input:    input,
			Expected: expected,
			Category: category,
		})
	}

	return cases, nil
}

func loadFromJSONL(filePath string) ([]EvaluationCase, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cases []EvaluationCase
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		var c EvaluationCase
		if err := json.Unmarshal(scanner.Bytes(), &c); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  JSON decode error: %v\n", err)
			continue
		}
		cases = append(cases, c)
	}

	return cases, scanner.Err()
}
