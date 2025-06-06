package cmd

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/trknhr/ghosttype/internal"
	"github.com/trknhr/ghosttype/model"
)

var Eval = &cobra.Command{
	Use:   "eval",
	Short: "Background worker to learn full shell history",
}

type EvalCase struct {
	Input    string `json:"input"`
	Expected string `json:"expected"`
}

func RunEvaluation(model model.SuggestModel, jsonlPath string) error {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var (
		top1Correct int
		top3Correct int
		total       int
	)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var c EvalCase
		if err := json.Unmarshal(scanner.Bytes(), &c); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  JSON decode error: %v\n", err)
			continue
		}

		suggestions, err := model.Predict(c.Input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Prediction error for input %q: %v\n", c.Input, err)
			continue
		}
		total++

		top1Match := len(suggestions) > 0 && suggestions[0].Text == c.Expected
		top3Match := false
		for _, s := range suggestions[:min(3, len(suggestions))] {
			if s.Text == c.Expected {
				top3Match = true
				break
			}
		}

		if top1Match {
			top1Correct++
			top3Correct++
		} else if top3Match {
			top3Correct++
			fmt.Printf("🟡 Top3 Hit | Input: %q | Expected: %q\n", c.Input, c.Expected)
			printSuggestionList(suggestions)
		} else {
			fmt.Printf("🔴 Missed  | Input: %q | Expected: %q\n", c.Input, c.Expected)
			printSuggestionList(suggestions)
		}
	}

	fmt.Printf("🔎 Evaluation result (%T)\n", model)
	fmt.Printf("Top1 Accuracy: %.2f%%\n", float64(top1Correct)/float64(total)*100)
	fmt.Printf("Top3 Accuracy: %.2f%%\n", float64(top3Correct)/float64(total)*100)
	return nil
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
		Short: "Evaluate model with JSONL test cases",
		RunE: func(cmd *cobra.Command, args []string) error {
			model := internal.GenerateModel(db, modelName)
			if model == nil {
				return fmt.Errorf("failed to create model for: %s", modelName)
			}
			return RunEvaluation(model, evalFile)
		},
	}

	cmd.Flags().StringVarP(&evalFile, "file", "f", "", "Path to JSONL evaluation file")
	cmd.Flags().StringVarP(&modelName, "model", "m", "freq", "Model name to evaluate (e.g. freq, markov)")
	cmd.MarkFlagRequired("file")

	return cmd
}
