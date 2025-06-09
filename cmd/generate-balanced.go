package cmd

import (
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type EvalDataSource struct {
	Name        string
	Weight      float64 // Percentage of total dataset
	Generator   func(int) ([]EvaluationCase, error)
	Description string
}

type PopularCommand struct {
	Command     string
	Description string
	Category    string
	Variations  []string
}

var generateBalancedCmd = &cobra.Command{
	Use:   "balanced",
	Short: "Generate balanced evaluation dataset from multiple sources",
	Example: `
  # Generate balanced dataset
  ghosttype generate balanced --output eval_balanced.csv --count 1000
  
  # Custom weights
  ghosttype generate balanced --output eval_balanced.csv --count 1000 \
    --history-weight 40 --popular-weight 40 --fuzzy-weight 20`,
	RunE: generateBalancedEvalData,
}

var (
	balancedOutput  string
	balancedCount   int
	historyWeight   float64
	popularWeight   float64
	fuzzyWeight     float64
	githubWeight    float64
	userHistoryFile string
)

func init() {
	generateBalancedCmd.Flags().StringVarP(&balancedOutput, "output", "o", "eval_balanced.csv", "Output CSV file")
	generateBalancedCmd.Flags().IntVarP(&balancedCount, "count", "c", 1000, "Total number of test cases")
	generateBalancedCmd.Flags().Float64Var(&historyWeight, "history-weight", 30.0, "Percentage from user history")
	generateBalancedCmd.Flags().Float64Var(&popularWeight, "popular-weight", 40.0, "Percentage from popular commands")
	generateBalancedCmd.Flags().Float64Var(&fuzzyWeight, "fuzzy-weight", 20.0, "Percentage from fuzzy patterns")
	generateBalancedCmd.Flags().Float64Var(&githubWeight, "github-weight", 10.0, "Percentage from GitHub commands")
	generateBalancedCmd.Flags().StringVar(&userHistoryFile, "history", "~/.zsh_history", "Path to user history file")

	generateCmd.AddCommand(generateBalancedCmd)
}

func generateBalancedEvalData(cmd *cobra.Command, args []string) error {
	fmt.Printf("🎯 Generating balanced evaluation dataset...\n")
	fmt.Printf("📊 Target: %d cases with weights: History(%g%%) Popular(%g%%) Fuzzy(%g%%) GitHub(%g%%)\n",
		balancedCount, historyWeight, popularWeight, fuzzyWeight, githubWeight)

	// Normalize weights
	totalWeight := historyWeight + popularWeight + fuzzyWeight + githubWeight
	historyWeight = historyWeight / totalWeight * 100
	popularWeight = popularWeight / totalWeight * 100
	fuzzyWeight = fuzzyWeight / totalWeight * 100
	githubWeight = githubWeight / totalWeight * 100

	sources := []EvalDataSource{
		{
			Name:        "user_history",
			Weight:      historyWeight,
			Generator:   generateFromUserHistory,
			Description: "Based on user's actual command history",
		},
		{
			Name:        "popular_commands",
			Weight:      popularWeight,
			Generator:   generateFromPopularCommands,
			Description: "Common commands used by developers",
		},
		{
			Name:        "fuzzy_patterns",
			Weight:      fuzzyWeight,
			Generator:   generateFuzzyPatterns,
			Description: "Realistic fuzzy search patterns",
		},
		{
			Name:        "github_commands",
			Weight:      githubWeight,
			Generator:   generateFromGitHubExamples,
			Description: "Commands from popular GitHub repositories",
		},
	}

	var allCases []EvaluationCase

	for _, source := range sources {
		count := int(float64(balancedCount) * source.Weight / 100)
		if count == 0 {
			continue
		}

		fmt.Printf("🔄 Generating %d cases from %s...\n", count, source.Name)
		cases, err := source.Generator(count)
		if err != nil {
			fmt.Printf("⚠️  Error with %s: %v\n", source.Name, err)
			continue
		}

		// Add source info
		for i := range cases {
			cases[i].Source = source.Name
		}

		allCases = append(allCases, cases...)
		fmt.Printf("✅ Generated %d cases from %s\n", len(cases), source.Name)
	}

	// Shuffle to mix sources
	rand.Shuffle(len(allCases), func(i, j int) {
		allCases[i], allCases[j] = allCases[j], allCases[i]
	})

	// Write to CSV
	err := writeBalancedCSV(allCases, balancedOutput)
	if err != nil {
		return fmt.Errorf("failed to write CSV: %w", err)
	}

	fmt.Printf("🎉 Successfully generated %d balanced test cases to %s\n", len(allCases), balancedOutput)
	printDatasetSummary(allCases)

	return nil
}

func generateFromUserHistory(count int) ([]EvaluationCase, error) {
	// Parse user's actual history
	commands, err := parseHistoryFile(expandPath(userHistoryFile))
	if err != nil {
		return nil, err
	}

	// Get frequent commands
	frequent := getFrequentCommands(commands, 2)
	if len(frequent) == 0 {
		return nil, fmt.Errorf("no frequent commands found in history")
	}

	var cases []EvaluationCase
	for _, cmdFreq := range frequent {
		if len(cases) >= count {
			break
		}

		cmd := cmdFreq.Command

		// Generate realistic partial inputs
		patterns := generateRealisticPatterns(cmd)
		for _, pattern := range patterns {
			if len(cases) >= count {
				break
			}

			cases = append(cases, EvaluationCase{
				Input:    pattern,
				Expected: cmd,
				Category: getCommandCategory(cmd),
			})
		}
	}

	return cases[:min(count, len(cases))], nil
}

func generateFromPopularCommands(count int) ([]EvaluationCase, error) {
	popular := getPopularCommands()
	var cases []EvaluationCase

	for _, cmd := range popular {
		if len(cases) >= count {
			break
		}

		// Generate variations
		patterns := generateRealisticPatterns(cmd.Command)
		for _, pattern := range patterns {
			if len(cases) >= count {
				break
			}

			cases = append(cases, EvaluationCase{
				Input:    pattern,
				Expected: cmd.Command,
				Category: cmd.Category,
			})
		}

		// Add variations if available
		for _, variation := range cmd.Variations {
			if len(cases) >= count {
				break
			}

			patterns := generateRealisticPatterns(variation)
			for _, pattern := range patterns {
				if len(cases) >= count {
					break
				}

				cases = append(cases, EvaluationCase{
					Input:    pattern,
					Expected: variation,
					Category: cmd.Category,
				})
			}
		}
	}

	return cases[:min(count, len(cases))], nil
}

func generateFuzzyPatterns(count int) ([]EvaluationCase, error) {
	commands := []string{
		"git status", "git commit -m", "git push origin", "git pull origin",
		"docker run", "docker ps", "docker build", "docker stop",
		"npm install", "npm start", "npm run build", "npm test",
		"ls -la", "cd ..", "mkdir -p", "rm -rf",
		"kubectl get pods", "kubectl describe", "kubectl apply -f",
		"python3 -m", "pip install", "go mod tidy", "go build",
	}

	var cases []EvaluationCase

	for _, cmd := range commands {
		if len(cases) >= count {
			break
		}

		// Generate fuzzy patterns
		fuzzyPatterns := []string{
			generateAbbreviation(cmd), // "git status" -> "gst"
			generateSkipChars(cmd, 1), // "git status" -> "gt status"
			generateSkipChars(cmd, 2), // "git status" -> "g status"
			generateTypo(cmd),         // "git status" -> "git statsu"
			generatePartialMatch(cmd), // "git status" -> "git stat"
			generateInitials(cmd),     // "git status" -> "gs"
			generatePrefixSkip(cmd),   // "git status" -> "git s"
		}

		for _, pattern := range fuzzyPatterns {
			if len(cases) >= count || pattern == "" || pattern == cmd {
				continue
			}

			cases = append(cases, EvaluationCase{
				Input:    pattern,
				Expected: cmd,
				Category: getCommandCategory(cmd),
			})
		}
	}

	return cases[:min(count, len(cases))], nil
}

func generateFromGitHubExamples(count int) ([]EvaluationCase, error) {
	// Simulate popular GitHub repository commands
	githubCommands := []string{
		"make build", "make test", "make install", "make clean",
		"cargo run", "cargo build", "cargo test", "cargo check",
		"mvn clean install", "mvn test", "mvn package",
		"gradle build", "gradle test", "gradle run",
		"yarn install", "yarn start", "yarn build", "yarn test",
		"terraform init", "terraform plan", "terraform apply",
		"ansible-playbook", "vagrant up", "vagrant ssh",
		"helm install", "helm upgrade", "helm list",
	}

	var cases []EvaluationCase

	for _, cmd := range githubCommands {
		if len(cases) >= count {
			break
		}

		patterns := generateRealisticPatterns(cmd)
		for _, pattern := range patterns {
			if len(cases) >= count {
				break
			}

			cases = append(cases, EvaluationCase{
				Input:    pattern,
				Expected: cmd,
				Category: getCommandCategory(cmd),
			})
		}
	}

	return cases[:min(count, len(cases))], nil
}

func generateRealisticPatterns(cmd string) []string {
	var patterns []string

	// Different realistic input patterns users actually type
	patterns = append(patterns,
		generateAbbreviation(cmd),
		generatePrefixPattern(cmd, 2),
		generatePrefixPattern(cmd, 3),
		generateSkipChars(cmd, 1),
		generateInitials(cmd),
		generatePartialMatch(cmd),
	)

	// Filter out empty or same patterns
	var filtered []string
	for _, p := range patterns {
		if p != "" && p != cmd && len(p) >= 2 {
			filtered = append(filtered, p)
		}
	}

	return filtered
}

func generateAbbreviation(cmd string) string {
	words := strings.Fields(cmd)
	if len(words) < 2 {
		return cmd[:min(3, len(cmd))]
	}

	// Take first letter of each word + first few chars of last word
	var abbrev strings.Builder
	for _, word := range words[:len(words)-1] {
		if len(word) > 0 {
			abbrev.WriteByte(word[0])
		}
	}
	lastWord := words[len(words)-1]
	abbrev.WriteString(lastWord[:min(2, len(lastWord))])

	return abbrev.String()
}

func generateSkipChars(cmd string, skip int) string {
	if len(cmd) <= skip {
		return cmd
	}

	result := ""
	for i, char := range cmd {
		if i%(skip+1) == 0 {
			result += string(char)
		}
	}
	return result
}

func generateTypo(cmd string) string {
	if len(cmd) < 4 {
		return cmd
	}

	// Simple typo: swap two adjacent characters
	pos := rand.Intn(len(cmd) - 1)
	runes := []rune(cmd)
	runes[pos], runes[pos+1] = runes[pos+1], runes[pos]
	return string(runes)
}

func generatePartialMatch(cmd string) string {
	words := strings.Fields(cmd)
	if len(words) == 0 {
		return cmd[:min(len(cmd)*3/4, len(cmd))]
	}

	// Take first word + partial second word
	if len(words) == 1 {
		return words[0][:min(len(words[0])*3/4, len(words[0]))]
	}

	secondWord := words[1][:min(len(words[1])/2+1, len(words[1]))]
	return words[0] + " " + secondWord
}

func generateInitials(cmd string) string {
	words := strings.Fields(cmd)
	if len(words) < 2 {
		return cmd[:min(2, len(cmd))]
	}

	var initials strings.Builder
	for _, word := range words {
		if len(word) > 0 {
			initials.WriteByte(word[0])
		}
	}
	return initials.String()
}

func generatePrefixPattern(cmd string, length int) string {
	if len(cmd) <= length {
		return cmd
	}
	return cmd[:length]
}

func generatePrefixSkip(cmd string) string {
	words := strings.Fields(cmd)
	if len(words) < 2 {
		return cmd
	}

	// First word + first char of second word
	return words[0] + " " + string(words[1][0])
}

func getPopularCommands() []PopularCommand {
	return []PopularCommand{
		{
			Command: "git status", Category: "git",
			Variations: []string{"git st", "git status --short"},
		},
		{
			Command: "git commit -m", Category: "git",
			Variations: []string{"git commit -am", "git commit --amend"},
		},
		{
			Command: "docker ps", Category: "docker",
			Variations: []string{"docker ps -a", "docker container ls"},
		},
		{
			Command: "ls -la", Category: "filesystem",
			Variations: []string{"ls -l", "ls -al", "ll"},
		},
		{
			Command: "npm install", Category: "npm",
			Variations: []string{"npm i", "npm install --save"},
		},
		// Add more popular commands...
	}
}

func writeBalancedCSV(cases []EvaluationCase, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"input", "expected", "category", "source"}); err != nil {
		return err
	}

	// Write data
	for _, c := range cases {
		record := []string{c.Input, c.Expected, c.Category, c.Source}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

func printDatasetSummary(cases []EvaluationCase) {
	fmt.Println("\n📊 DATASET SUMMARY")
	fmt.Println("═══════════════════════════════════════")

	// Count by source
	sourceCount := make(map[string]int)
	categoryCount := make(map[string]int)

	for _, c := range cases {
		sourceCount[c.Source]++
		categoryCount[c.Category]++
	}

	fmt.Println("By Source:")
	for source, count := range sourceCount {
		fmt.Printf("  %-15s: %3d cases (%.1f%%)\n",
			source, count, float64(count)/float64(len(cases))*100)
	}

	fmt.Println("\nBy Category:")
	for category, count := range categoryCount {
		fmt.Printf("  %-12s: %3d cases\n", category, count)
	}

	fmt.Printf("\nTotal: %d test cases\n", len(cases))
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return strings.Replace(path, "~", home, 1)
	}
	return path
}
