// package cmd

// import (
// 	"bufio"
// 	"database/sql"
// 	"encoding/json"
// 	"fmt"
// 	"os"

// 	"github.com/spf13/cobra"
// 	"github.com/trknhr/ghosttype/internal"
// 	"github.com/trknhr/ghosttype/model"
// )

// var Eval = &cobra.Command{
// 	Use:   "eval",
// 	Short: "Background worker to learn full shell history",
// }

// type EvaluationCase struct {
// 	Input    string `json:"input"`
// 	Expected string `json:"expected"`
// }

// func RunEvaluation(model model.SuggestModel, jsonlPath string) error {
// 	file, err := os.Open(jsonlPath)
// 	if err != nil {
// 		return err
// 	}
// 	defer file.Close()

// 	var (
// 		top1Correct int
// 		top3Correct int
// 		total       int
// 	)

// 	scanner := bufio.NewScanner(file)
// 	for scanner.Scan() {
// 		var c EvaluationCase
// 		if err := json.Unmarshal(scanner.Bytes(), &c); err != nil {
// 			fmt.Fprintf(os.Stderr, "⚠️  JSON decode error: %v\n", err)
// 			continue
// 		}

// 		suggestions, err := model.Predict(c.Input)
// 		if err != nil {
// 			fmt.Fprintf(os.Stderr, "⚠️  Prediction error for input %q: %v\n", c.Input, err)
// 			continue
// 		}
// 		total++

// 		top1Match := len(suggestions) > 0 && suggestions[0].Text == c.Expected
// 		top3Match := false
// 		for _, s := range suggestions[:min(3, len(suggestions))] {
// 			if s.Text == c.Expected {
// 				top3Match = true
// 				break
// 			}
// 		}

// 		if top1Match {
// 			top1Correct++
// 			top3Correct++
// 		} else if top3Match {
// 			top3Correct++
// 			fmt.Printf("🟡 Top3 Hit | Input: %q | Expected: %q\n", c.Input, c.Expected)
// 			printSuggestionList(suggestions)
// 		} else {
// 			fmt.Printf("🔴 Missed  | Input: %q | Expected: %q\n", c.Input, c.Expected)
// 			printSuggestionList(suggestions)
// 		}
// 	}

// 	fmt.Printf("🔎 Evaluation result (%T)\n", model)
// 	fmt.Printf("Top1 Accuracy: %.2f%%\n", float64(top1Correct)/float64(total)*100)
// 	fmt.Printf("Top3 Accuracy: %.2f%%\n", float64(top3Correct)/float64(total)*100)
// 	return nil
// }

// func printSuggestionList(suggestions []model.Suggestion) {
// 	for i, s := range suggestions[:min(5, len(suggestions))] {
// 		fmt.Printf("  [%d] %s\n", i+1, s.Text)
// 	}
// }

// func NewEvalCmd(db *sql.DB) *cobra.Command {
// 	var evalFile string
// 	var modelName string

// 	cmd := &cobra.Command{
// 		Use:   "eval",
// 		Short: "Evaluate model with JSONL test cases",
// 		RunE: func(cmd *cobra.Command, args []string) error {
// 			model := internal.GenerateModel(db, modelName)
// 			if model == nil {
// 				return fmt.Errorf("failed to create model for: %s", modelName)
// 			}
// 			return RunEvaluation(model, evalFile)
// 		},
// 	}

// 	cmd.Flags().StringVarP(&evalFile, "file", "f", "", "Path to JSONL evaluation file")
// 	cmd.Flags().StringVarP(&modelName, "model", "m", "freq", "Model name to evaluate (e.g. freq, markov)")
// 	cmd.MarkFlagRequired("file")

// 	return cmd
// }

package cmd

import (
	"bufio"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trknhr/ghosttype/internal"
	"github.com/trknhr/ghosttype/model"
)

var Eval = &cobra.Command{
	Use:   "eval",
	Short: "Background worker to learn full shell history",
}

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

func RunEvaluation(model model.SuggestModel, filePath string) error {
	cases, err := loadEvaluationCases(filePath)
	if err != nil {
		return fmt.Errorf("failed to load evaluation cases: %w", err)
	}

	fmt.Printf("📊 Loaded %d evaluation cases\n", len(cases))

	result := EvaluationResult{
		ByCategory: make(map[string]CategoryResult),
	}

	for _, c := range cases {
		suggestions, err := model.Predict(c.Input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Prediction error for input %q: %v\n", c.Input, err)
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
		for _, s := range suggestions[:min(10, len(suggestions))] {
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
			fmt.Printf("🟡 Top3 Hit | Input: %q | Expected: %q | Category: %s\n", c.Input, c.Expected, c.Category)
			printSuggestionList(suggestions)
		} else {
			fmt.Printf("🔴 Missed  | Input: %q | Expected: %q | Category: %s\n", c.Input, c.Expected, c.Category)
			printSuggestionList(suggestions)
		}

		result.ByCategory[c.Category] = categoryResult
	}

	printEvaluationResults(model, result)
	return nil
}

func loadEvaluationCases(filePath string) ([]EvaluationCase, error) {
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

func printEvaluationResults(model model.SuggestModel, result EvaluationResult) {
	fmt.Printf("\n🔎 Evaluation Results (%T)\n", model)
	fmt.Printf("═══════════════════════════════════════\n")
	fmt.Printf("Overall Performance:\n")
	fmt.Printf("  Total Cases: %d\n", result.Total)
	fmt.Printf("  Top-1 Accuracy: %.2f%% (%d/%d)\n",
		float64(result.Top1Correct)/float64(result.Total)*100,
		result.Top1Correct, result.Total)
	fmt.Printf("  Top-3 Accuracy: %.2f%% (%d/%d)\n",
		float64(result.Top3Correct)/float64(result.Total)*100,
		result.Top3Correct, result.Total)

	if len(result.ByCategory) > 1 {
		fmt.Printf("\nBy Category:\n")
		for category, catResult := range result.ByCategory {
			if catResult.Total == 0 {
				continue
			}
			fmt.Printf("  %s:\n", category)
			fmt.Printf("    Cases: %d\n", catResult.Total)
			fmt.Printf("    Top-1: %.2f%% (%d/%d)\n",
				float64(catResult.Top1Correct)/float64(catResult.Total)*100,
				catResult.Top1Correct, catResult.Total)
			fmt.Printf("    Top-3: %.2f%% (%d/%d)\n",
				float64(catResult.Top3Correct)/float64(catResult.Total)*100,
				catResult.Top3Correct, catResult.Total)
		}
	}
}

func printSuggestionList(suggestions []model.Suggestion) {
	for i, s := range suggestions[:min(5, len(suggestions))] {
		fmt.Printf("  [%d] %s\n", i+1, s.Text)
	}
}

func NewEvalCmd(db *sql.DB) *cobra.Command {
	var evalFile string
	var modelName string

	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Evaluate model with CSV or JSONL test cases",
		Example: `
  # Evaluate with CSV file
  ghosttype eval -f eval_data.csv -m freq
  
  # Evaluate with JSONL file  
  ghosttype eval -f eval_data.jsonl -m markov`,
		RunE: func(cmd *cobra.Command, args []string) error {
			model := internal.GenerateModel(db, modelName)
			if model == nil {
				return fmt.Errorf("failed to create model for: %s", modelName)
			}
			return RunEvaluation(model, evalFile)
		},
	}

	cmd.Flags().StringVarP(&evalFile, "file", "f", "", "Path to CSV or JSONL evaluation file")
	cmd.Flags().StringVarP(&modelName, "model", "m", "freq", "Model name to evaluate (e.g. freq, markov)")
	cmd.MarkFlagRequired("file")

	return cmd
}
