// cmd/generate.go
package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type EvalCase struct {
	Input    string `json:"input"`
	Expected string `json:"expected"`
	Source   string `json:"source"`
	Category string `json:"category,omitempty"`
}

type CommandFreq struct {
	Command string
	Count   int
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate evaluation data from history",
	Long:  "Generate evaluation test cases from command history automatically",
}

var generateEvalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Generate evaluation test cases from history file",
	Example: `
  # Generate from zsh history
  ghosttype generate eval --history ~/.zsh_history --output eval_auto.jsonl --count 500
  
  # Generate from bash history  
  ghosttype generate eval --history ~/.bash_history --output eval_auto.jsonl --count 1000
  
  # Generate with minimum frequency filter
  ghosttype generate eval --history ~/.zsh_history --output eval_auto.jsonl --min-freq 3`,
	Run: generateEvalData,
}

var (
	historyFile string
	outputFile  string
	maxCount    int
	minFreq     int
	categories  []string
)

func init() {
	generateEvalCmd.Flags().StringVarP(&historyFile, "history", "H", "", "Path to history file (required)")
	generateEvalCmd.Flags().StringVarP(&outputFile, "output", "o", "eval_generated.jsonl", "Output JSONL file")
	generateEvalCmd.Flags().IntVarP(&maxCount, "count", "c", 500, "Maximum number of test cases to generate")
	generateEvalCmd.Flags().IntVarP(&minFreq, "min-freq", "f", 2, "Minimum frequency for commands to include")
	generateEvalCmd.Flags().StringSliceVar(&categories, "categories", []string{}, "Filter by command categories (git,docker,npm,etc)")

	generateEvalCmd.MarkFlagRequired("history")

	generateCmd.AddCommand(generateEvalCmd)
}

func generateEvalData(cmd *cobra.Command, args []string) {
	fmt.Printf("Generating eval data from %s...\n", historyFile)

	// 1. Parse history file
	commands, err := parseHistoryFile(historyFile)
	if err != nil {
		fmt.Printf("Error parsing history: %v\n", err)
		return
	}
	fmt.Printf("Parsed %d commands from history\n", len(commands))

	// 2. Get frequent commands
	frequent := getFrequentCommands(commands, minFreq)
	fmt.Printf("Found %d frequent commands (min frequency: %d)\n", len(frequent), minFreq)

	// 3. Filter by categories if specified
	if len(categories) > 0 {
		frequent = filterByCategories(frequent, categories)
		fmt.Printf("Filtered to %d commands matching categories: %v\n", len(frequent), categories)
	}

	// 4. Generate test cases
	testCases := generateTestCases(frequent, maxCount)
	fmt.Printf("Generated %d test cases\n", len(testCases))

	// 5. Write to output file
	err = writeTestCases(testCases, outputFile)
	if err != nil {
		fmt.Printf("Error writing output: %v\n", err)
		return
	}

	fmt.Printf("✅ Successfully generated %d test cases to %s\n", len(testCases), outputFile)

	// 6. Show summary
	showSummary(testCases)
}

func parseHistoryFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var commands []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle zsh extended history format: : 1234567890:0;command
		if strings.Contains(line, ";") && strings.HasPrefix(line, ":") {
			parts := strings.SplitN(line, ";", 2)
			if len(parts) == 2 {
				line = parts[1]
			}
		}

		// Clean up the command
		line = strings.TrimSpace(line)
		if line != "" && len(line) > 2 {
			commands = append(commands, line)
		}
	}

	return commands, scanner.Err()
}

func getFrequentCommands(commands []string, minFreq int) []CommandFreq {
	freq := make(map[string]int)

	for _, cmd := range commands {
		freq[cmd]++
	}

	var frequent []CommandFreq
	for cmd, count := range freq {
		if count >= minFreq {
			frequent = append(frequent, CommandFreq{
				Command: cmd,
				Count:   count,
			})
		}
	}

	// Sort by frequency (descending)
	sort.Slice(frequent, func(i, j int) bool {
		return frequent[i].Count > frequent[j].Count
	})

	return frequent
}

func filterByCategories(commands []CommandFreq, categories []string) []CommandFreq {
	var filtered []CommandFreq

	for _, cmd := range commands {
		for _, category := range categories {
			if strings.HasPrefix(cmd.Command, category+" ") || cmd.Command == category {
				filtered = append(filtered, cmd)
				break
			}
		}
	}

	return filtered
}

func generateTestCases(frequent []CommandFreq, maxCount int) []EvalCase {
	var testCases []EvalCase

	for _, cmdFreq := range frequent {
		cmd := cmdFreq.Command

		// Generate partial string test cases
		for i := 2; i < len(cmd) && len(testCases) < maxCount; i++ {
			input := cmd[:i]

			// Skip if input is too short or same as expected
			if len(input) < 2 || input == cmd {
				continue
			}

			testCase := EvalCase{
				Input:    input,
				Expected: cmd,
				Source:   "history_analysis",
				Category: getCommandCategory(cmd),
			}

			testCases = append(testCases, testCase)
		}

		if len(testCases) >= maxCount {
			break
		}
	}

	return testCases
}

func getCommandCategory(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "unknown"
	}

	baseCmd := parts[0]

	// Common categories
	categories := map[string]string{
		"git":    "git",
		"docker": "docker",
		"npm":    "npm",
		"yarn":   "yarn",
		"go":     "go",
		"python": "python",
		"pip":    "python",
		"curl":   "network",
		"wget":   "network",
		"ssh":    "network",
		"ls":     "filesystem",
		"cd":     "filesystem",
		"mkdir":  "filesystem",
		"rm":     "filesystem",
		"cp":     "filesystem",
		"mv":     "filesystem",
	}

	if category, exists := categories[baseCmd]; exists {
		return category
	}

	return "other"
}

func writeTestCases(testCases []EvalCase, outputFile string) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, testCase := range testCases {
		if err := encoder.Encode(testCase); err != nil {
			return err
		}
	}

	return nil
}

func showSummary(testCases []EvalCase) {
	fmt.Println("\n📊 Generation Summary:")

	// Count by category
	categoryCount := make(map[string]int)
	for _, tc := range testCases {
		categoryCount[tc.Category]++
	}

	fmt.Println("Categories:")
	for category, count := range categoryCount {
		fmt.Printf("  %s: %d cases\n", category, count)
	}

	// Show sample cases
	fmt.Println("\n🔍 Sample test cases:")
	for i, tc := range testCases[:min(5, len(testCases))] {
		fmt.Printf("  %d. \"%s\" → \"%s\" [%s]\n", i+1, tc.Input, tc.Expected, tc.Category)
	}

	if len(testCases) > 5 {
		fmt.Printf("  ... and %d more cases\n", len(testCases)-5)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
