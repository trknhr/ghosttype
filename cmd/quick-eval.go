package cmd

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/trknhr/ghosttype/internal/history"
	"github.com/trknhr/ghosttype/internal/model"
	"github.com/trknhr/ghosttype/internal/ollama"
	"github.com/trknhr/ghosttype/internal/store"
)

func NewQuickEvalCmd(db *sql.DB) *cobra.Command {
	var evalFile string
	var sampleSize int

	cmd := &cobra.Command{
		Use:   "quick-eval",
		Short: "Quick ensemble evaluation for development",
		Long: `Fast evaluation using a sample of test cases for rapid iteration
during development. Shows key metrics without detailed breakdowns.`,
		Example: `
  # Quick check with 50 cases
  ghosttype quick-eval -f eval_balanced.csv --sample 50
  
  # Full quick evaluation
  ghosttype quick-eval -f eval_balanced.csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunQuickEvaluation(db, evalFile, sampleSize)
		},
	}

	cmd.Flags().StringVarP(&evalFile, "file", "f", "", "Path to CSV or JSONL evaluation file")
	cmd.Flags().IntVar(&sampleSize, "sample", 100, "Number of test cases to sample (0 = all)")
	cmd.MarkFlagRequired("file")

	return cmd
}

func RunQuickEvaluation(db *sql.DB, filePath string, sampleSize int) error {
	start := time.Now()

	// Load test cases
	cases, err := loadEvaluationCases(filePath)
	if err != nil {
		return fmt.Errorf("failed to load evaluation cases: %w", err)
	}

	// Sample if requested
	if sampleSize > 0 && sampleSize < len(cases) {
		cases = cases[:sampleSize]
	}

	fmt.Printf("⚡ QUICK ENSEMBLE EVALUATION\n")
	fmt.Printf("════════════════════════════════════\n")
	fmt.Printf("📊 Evaluating %d test cases...\n", len(cases))

	// Create ensemble model
	historyStore := store.NewSQLHistoryStore(db)
	hitoryLoader := history.NewHistoryLoaderAuto()
	ollamaClient := ollama.NewHTTPClient("llama3.2:1b", "nomic-embed-text")
	ensembleModel, events, _ := model.GenerateModel(historyStore, hitoryLoader, ollamaClient, db, "")
	model.DrainAndLogEvents(events)
	if ensembleModel == nil {
		return fmt.Errorf("failed to create ensemble model")
	}

	var total, top1, top3, top10 int

	for i, testCase := range cases {
		suggestions, err := ensembleModel.Predict(testCase.Input)
		if err != nil {
			continue
		}

		total++

		// Check ranks (simplified)
		for rank, suggestion := range suggestions[:min(10, len(suggestions))] {
			if suggestion.Text == testCase.Expected {
				if rank == 0 {
					top1++
				}
				if rank < 3 {
					top3++
				}
				top10++
				break
			}
		}

		// Progress indicator
		if i%25 == 0 && i > 0 {
			fmt.Printf("  Progress: %d/%d\r", i, len(cases))
		}
	}

	duration := time.Since(start)

	// Results
	fmt.Printf("\n🎯 RESULTS:\n")
	fmt.Printf("  Top-1:  %3d/%3d (%5.1f%%) %s\n",
		top1, total, float64(top1)/float64(total)*100,
		getQuickRating(float64(top1)/float64(total)*100, 25, 15))
	fmt.Printf("  Top-3:  %3d/%3d (%5.1f%%) %s\n",
		top3, total, float64(top3)/float64(total)*100,
		getQuickRating(float64(top3)/float64(total)*100, 50, 30))
	fmt.Printf("  Top-10: %3d/%3d (%5.1f%%) %s\n",
		top10, total, float64(top10)/float64(total)*100,
		getQuickRating(float64(top10)/float64(total)*100, 70, 50))

	fmt.Printf("\n⏱️  Time: %v | Rate: %.1f cases/sec\n",
		duration.Round(time.Millisecond),
		float64(total)/duration.Seconds())

	return nil
}

func getQuickRating(rate, good, fair float64) string {
	if rate >= good {
		return "✅"
	} else if rate >= fair {
		return "⚠️ "
	} else {
		return "❌"
	}
}
