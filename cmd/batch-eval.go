package cmd

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/trknhr/ghosttype/internal"
	"github.com/trknhr/ghosttype/model"
)

type ModelResult struct {
	ModelName string
	Duration  time.Duration
	Result    EvaluationResult
}

func NewBatchEvalCmd(db *sql.DB) *cobra.Command {
	var evalFile string
	var models []string

	cmd := &cobra.Command{
		Use:   "batch-eval",
		Short: "Evaluate multiple models at once and compare results",
		Example: `
  # Evaluate all models
  ghosttype batch-eval -f eval_data.csv
  
  # Evaluate specific models
  ghosttype batch-eval -f eval_data.csv -m freq,embedding,llm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunBatchEvaluation(db, evalFile, models)
		},
	}

	cmd.Flags().StringVarP(&evalFile, "file", "f", "", "Path to CSV or JSONL evaluation file")
	cmd.Flags().StringSliceVarP(&models, "models", "m", []string{"freq", "embedding", "llm", "prefix"}, "Models to evaluate (comma-separated)")
	cmd.MarkFlagRequired("file")

	return cmd
}

func RunBatchEvaluation(db *sql.DB, filePath string, modelNames []string) error {
	// Load test cases once
	cases, err := loadEvaluationCases(filePath)
	if err != nil {
		return fmt.Errorf("failed to load evaluation cases: %w", err)
	}

	fmt.Printf("📊 Loaded %d evaluation cases\n", len(cases))
	fmt.Printf("🔄 Testing %d models: %s\n", len(modelNames), strings.Join(modelNames, ", "))
	fmt.Println("═══════════════════════════════════════")

	var results []ModelResult

	for _, modelName := range modelNames {
		fmt.Printf("\n🧪 Testing model: %s\n", modelName)

		model := internal.GenerateModel(db, modelName)
		if model == nil {
			fmt.Printf("❌ Failed to create model: %s\n", modelName)
			continue
		}

		start := time.Now()
		result, err := runSingleModelEvaluation(model, cases)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("❌ Error evaluating %s: %v\n", modelName, err)
			continue
		}

		results = append(results, ModelResult{
			ModelName: modelName,
			Duration:  duration,
			Result:    result,
		})

		// Print individual results
		fmt.Printf("⏱️  Duration: %v\n", duration)
		fmt.Printf("🎯 Top-1: %.2f%% | Top-3: %.2f%%\n",
			float64(result.Top1Correct)/float64(result.Total)*100,
			float64(result.Top3Correct)/float64(result.Total)*100)
	}

	// Print comparison table
	printComparisonTable(results)

	return nil
}

func runSingleModelEvaluation(model model.SuggestModel, cases []EvaluationCase) (EvaluationResult, error) {
	result := EvaluationResult{
		ByCategory: make(map[string]CategoryResult),
	}

	for _, c := range cases {
		suggestions, err := model.Predict(c.Input)
		if err != nil {
			// Log error but continue
			fmt.Printf("⚠️  Prediction error for %q: %v\n", c.Input, err)
			continue
		}

		result.Total++

		// Initialize category if not exists
		if _, exists := result.ByCategory[c.Category]; !exists {
			result.ByCategory[c.Category] = CategoryResult{}
		}
		categoryResult := result.ByCategory[c.Category]
		categoryResult.Total++

		// Check matches
		top1Match := len(suggestions) > 0 && suggestions[0].Text == c.Expected
		top3Match := false
		for _, s := range suggestions[:min(3, len(suggestions))] {
			if s.Text == c.Expected {
				top3Match = true
				break
			}
		}

		if top1Match {
			result.Top1Correct++
			result.Top3Correct++
			categoryResult.Top1Correct++
			categoryResult.Top3Correct++
		} else if top3Match {
			result.Top3Correct++
			categoryResult.Top3Correct++
		}

		result.ByCategory[c.Category] = categoryResult
	}

	return result, nil
}

func printComparisonTable(results []ModelResult) {
	if len(results) == 0 {
		return
	}

	fmt.Println("\n📈 COMPARISON RESULTS")
	fmt.Println("═══════════════════════════════════════════════════════════")

	// Header
	fmt.Printf("%-12s | %-8s | %-8s | %-10s | %s\n",
		"Model", "Top-1", "Top-3", "Duration", "Performance")
	fmt.Println("─────────────┼──────────┼──────────┼────────────┼─────────────")

	// Find best performers
	bestTop1 := 0.0
	bestTop3 := 0.0
	fastestDuration := time.Hour

	for _, r := range results {
		top1Rate := float64(r.Result.Top1Correct) / float64(r.Result.Total) * 100
		top3Rate := float64(r.Result.Top3Correct) / float64(r.Result.Total) * 100

		if top1Rate > bestTop1 {
			bestTop1 = top1Rate
		}
		if top3Rate > bestTop3 {
			bestTop3 = top3Rate
		}
		if r.Duration < fastestDuration {
			fastestDuration = r.Duration
		}
	}

	// Print results with highlights
	for _, r := range results {
		top1Rate := float64(r.Result.Top1Correct) / float64(r.Result.Total) * 100
		top3Rate := float64(r.Result.Top3Correct) / float64(r.Result.Total) * 100

		// Add markers for best performance
		top1Marker := ""
		top3Marker := ""
		speedMarker := ""

		if top1Rate == bestTop1 {
			top1Marker = " 🥇"
		}
		if top3Rate == bestTop3 {
			top3Marker = " 🥇"
		}
		if r.Duration == fastestDuration {
			speedMarker = " ⚡"
		}

		performance := getPerformanceRating(top1Rate, top3Rate, r.Duration)

		fmt.Printf("%-12s | %6.2f%%%s | %6.2f%%%s | %8v%s | %s\n",
			r.ModelName,
			top1Rate, top1Marker,
			top3Rate, top3Marker,
			r.Duration.Round(time.Millisecond), speedMarker,
			performance)
	}

	fmt.Println("─────────────┴──────────┴──────────┴────────────┴─────────────")

	// Category breakdown for best model
	if len(results) > 0 {
		bestModel := findBestModel(results)
		fmt.Printf("\n📊 Category Breakdown (Best Model: %s)\n", bestModel.ModelName)
		printCategoryBreakdown(bestModel.Result)
	}
}

func getPerformanceRating(top1, top3 float64, duration time.Duration) string {
	// Simple performance rating based on accuracy and speed
	score := (top1*0.6 + top3*0.4) / 100 // 0-1 range

	if duration > time.Second {
		score *= 0.8 // Penalty for slow models
	}

	if score >= 0.9 {
		return "🌟 Excellent"
	} else if score >= 0.8 {
		return "✅ Very Good"
	} else if score >= 0.7 {
		return "👍 Good"
	} else if score >= 0.6 {
		return "⚠️  Fair"
	} else {
		return "❌ Poor"
	}
}

func findBestModel(results []ModelResult) ModelResult {
	best := results[0]
	bestScore := float64(best.Result.Top1Correct) / float64(best.Result.Total)

	for _, r := range results[1:] {
		score := float64(r.Result.Top1Correct) / float64(r.Result.Total)
		if score > bestScore {
			best = r
			bestScore = score
		}
	}

	return best
}

func printCategoryBreakdown(result EvaluationResult) {
	for category, catResult := range result.ByCategory {
		if catResult.Total == 0 {
			continue
		}
		top1Rate := float64(catResult.Top1Correct) / float64(catResult.Total) * 100
		top3Rate := float64(catResult.Top3Correct) / float64(catResult.Total) * 100

		fmt.Printf("  %-12s: Top-1 %5.1f%% | Top-3 %5.1f%% (%d cases)\n",
			category, top1Rate, top3Rate, catResult.Total)
	}
}
