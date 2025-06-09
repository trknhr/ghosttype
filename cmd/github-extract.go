package cmd

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type GitHubExtractor struct {
	Token    string
	Client   *http.Client
	BaseURL  string
	Cache    map[string][]byte // Simple file cache
	CacheDir string
}

type Repository struct {
	FullName        string `json:"full_name"`
	StargazersCount int    `json:"stargazers_count"`
	Language        string `json:"language"`
	DefaultBranch   string `json:"default_branch"`
}

type SearchResponse struct {
	Items      []Repository `json:"items"`
	TotalCount int          `json:"total_count"`
}

type FileContent struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
	Size     int    `json:"size"`
}

type CommandFrequency struct {
	Command   string   `json:"command"`
	Count     int      `json:"count"`
	Sources   []string `json:"sources"`
	Category  string   `json:"category"`
	Frequency float64  `json:"frequency"`
}

type ExtractionStats struct {
	ReposProcessed int            `json:"repos_processed"`
	FilesProcessed int            `json:"files_processed"`
	CommandsFound  int            `json:"commands_found"`
	ErrorCount     int            `json:"error_count"`
	ProcessingTime time.Duration  `json:"processing_time"`
	LanguageStats  map[string]int `json:"language_stats"`
	FileTypeStats  map[string]int `json:"file_type_stats"`
}

var extractGitHubCmd = &cobra.Command{
	Use:   "github-extract",
	Short: "Extract commands from popular GitHub repositories",
	Long: `Extract shell commands, build commands, and development workflows 
from popular GitHub repositories using the GitHub API. This creates a 
realistic dataset of commands that developers actually use.`,
	Example: `
  # Extract from top repositories
  ghosttype generate github-extract --token YOUR_TOKEN --output github_commands.json
  
  # Target specific languages with custom limits
  ghosttype generate github-extract --token YOUR_TOKEN \
    --languages go,python,javascript --max-repos 50 --min-stars 5000
  
  # Use cache directory for faster subsequent runs
  ghosttype generate github-extract --token YOUR_TOKEN \
    --cache-dir ./github_cache --output commands.json`,
	RunE: runGitHubExtraction,
}

var (
	githubToken     string
	githubOutput    string
	githubLanguages []string
	maxRepos        int
	minStars        int
	cacheDir        string
	skipCache       bool
	verbose         bool
	maxFileSize     int
)

func init() {
	extractGitHubCmd.Flags().StringVar(&githubToken, "token", "", "GitHub API token (or set GITHUB_TOKEN env)")
	extractGitHubCmd.Flags().StringVarP(&githubOutput, "output", "o", "github_commands.json", "Output JSON file")
	extractGitHubCmd.Flags().StringSliceVar(&githubLanguages, "languages",
		[]string{"go", "python", "javascript", "typescript", "rust", "java"},
		"Languages to target")
	extractGitHubCmd.Flags().IntVar(&maxRepos, "max-repos", 50, "Maximum repositories per language")
	extractGitHubCmd.Flags().IntVar(&minStars, "min-stars", 1000, "Minimum stars for repository")
	extractGitHubCmd.Flags().StringVar(&cacheDir, "cache-dir", "./github_cache", "Cache directory for API responses")
	extractGitHubCmd.Flags().BoolVar(&skipCache, "skip-cache", false, "Skip cache and fetch fresh data")
	extractGitHubCmd.Flags().BoolVar(&verbose, "verbose", false, "Verbose output")
	extractGitHubCmd.Flags().IntVar(&maxFileSize, "max-file-size", 50000, "Maximum file size to process (bytes)")

	generateCmd.AddCommand(extractGitHubCmd)
}

func runGitHubExtraction(cmd *cobra.Command, args []string) error {
	start := time.Now()

	// Setup
	if githubToken == "" {
		githubToken = os.Getenv("GITHUB_TOKEN")
	}
	if githubToken == "" {
		return fmt.Errorf("GitHub token required. Get one at https://github.com/settings/tokens")
	}

	// Create cache directory
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	extractor := &GitHubExtractor{
		Token:    githubToken,
		Client:   &http.Client{Timeout: 300 * time.Second},
		BaseURL:  "https://api.github.com",
		Cache:    make(map[string][]byte),
		CacheDir: cacheDir,
	}

	stats := &ExtractionStats{
		LanguageStats: make(map[string]int),
		FileTypeStats: make(map[string]int),
	}

	fmt.Printf("🚀 GitHub Command Extractor\n")
	fmt.Printf("═══════════════════════════════════════\n")
	fmt.Printf("🎯 Languages: %v\n", githubLanguages)
	fmt.Printf("⭐ Min stars: %d, Max repos per language: %d\n", minStars, maxRepos)
	fmt.Printf("💾 Cache: %s (skip: %v)\n", cacheDir, skipCache)
	fmt.Printf("📏 Max file size: %d bytes\n", maxFileSize)

	allCommands := make(map[string]*CommandFrequency)

	// Process each language
	for _, language := range githubLanguages {
		fmt.Printf("\n🔍 Processing %s repositories...\n", language)

		repos, err := extractor.searchRepositories(language, minStars, maxRepos)
		if err != nil {
			fmt.Printf("❌ Error searching %s repos: %v\n", language, err)
			stats.ErrorCount++
			continue
		}

		fmt.Printf("📦 Found %d popular %s repositories\n", len(repos), language)
		stats.LanguageStats[language] = len(repos)

		// Process repositories
		for i, repo := range repos {
			if verbose {
				fmt.Printf("  [%d/%d] %s (%d ⭐)\n",
					i+1, len(repos), repo.FullName, repo.StargazersCount)
			}

			commands, err := extractor.extractCommandsFromRepo(repo, stats)
			if err != nil {
				if verbose {
					fmt.Printf("    ⚠️  Error: %v\n", err)
				}
				stats.ErrorCount++
				continue
			}

			// Merge commands
			for cmdText, freq := range commands {
				if existing, exists := allCommands[cmdText]; exists {
					existing.Count += freq.Count
					existing.Sources = append(existing.Sources, freq.Sources...)
				} else {
					allCommands[cmdText] = &CommandFrequency{
						Command:  cmdText,
						Count:    freq.Count,
						Sources:  freq.Sources,
						Category: freq.Category,
					}
				}
			}

			stats.ReposProcessed++

			// Rate limiting - be nice to GitHub
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Calculate frequencies and sort
	totalCommands := 0
	for _, freq := range allCommands {
		totalCommands += freq.Count
	}

	var sortedCommands []CommandFrequency
	for _, freq := range allCommands {
		freq.Frequency = float64(freq.Count) / float64(totalCommands) * 100
		sortedCommands = append(sortedCommands, *freq)
	}

	sort.Slice(sortedCommands, func(i, j int) bool {
		return sortedCommands[i].Count > sortedCommands[j].Count
	})

	stats.CommandsFound = len(sortedCommands)
	stats.ProcessingTime = time.Since(start)

	// Save results
	err := saveExtractionResults(sortedCommands, stats, githubOutput)
	if err != nil {
		return fmt.Errorf("failed to save results: %w", err)
	}

	// Print summary
	printExtractionSummary(sortedCommands, stats)

	return nil
}

func (g *GitHubExtractor) searchRepositories(language string, minStars, maxRepos int) ([]Repository, error) {
	cacheKey := fmt.Sprintf("search_%s_%d_%d", language, minStars, maxRepos)

	// Check cache first
	if !skipCache {
		if cached, err := g.loadFromCache(cacheKey); err == nil {
			var repos []Repository
			if json.Unmarshal(cached, &repos) == nil {
				return repos, nil
			}
		}
	}

	query := fmt.Sprintf("language:%s stars:>%d", language, minStars)
	url := fmt.Sprintf("%s/search/repositories?q=%s&sort=stars&order=desc&per_page=100", g.BaseURL, query)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "token "+g.Token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "ghosttype-command-extractor")

	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	var searchResp SearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, err
	}

	// Limit results
	repos := searchResp.Items
	if len(repos) > maxRepos {
		repos = repos[:maxRepos]
	}

	// Cache the results
	if repoData, err := json.Marshal(repos); err == nil {
		g.saveToCache(cacheKey, repoData)
	}

	return repos, nil
}

func (g *GitHubExtractor) extractCommandsFromRepo(repo Repository, stats *ExtractionStats) (map[string]*CommandFrequency, error) {
	commands := make(map[string]*CommandFrequency)

	// Define target files by priority
	targetFiles := []struct {
		Path     string
		Parser   func(string, string) map[string]int
		Priority int
	}{
		{"Makefile", parseMakefileCommands, 3},
		{"package.json", parsePackageJsonCommands, 3},
		{"Dockerfile", parseDockerfileCommands, 2},
		{"docker-compose.yml", parseDockerComposeCommands, 2},
		{"docker-compose.yaml", parseDockerComposeCommands, 2},
		{".github/workflows/ci.yml", parseGitHubActionsCommands, 2},
		{".github/workflows/main.yml", parseGitHubActionsCommands, 2},
		{".github/workflows/test.yml", parseGitHubActionsCommands, 2},
		{"scripts/build.sh", parseShellCommands, 1},
		{"scripts/test.sh", parseShellCommands, 1},
		{"README.md", parseReadmeCommands, 1},
	}

	for _, target := range targetFiles {
		content, err := g.getFileContent(repo.FullName, target.Path)
		if err != nil {
			continue // File doesn't exist or error
		}

		if len(content) > maxFileSize {
			if verbose {
				fmt.Printf("    📄 Skipping %s (too large: %d bytes)\n", target.Path, len(content))
			}
			continue
		}

		fileCommands := target.Parser(string(content), target.Path)

		for cmd, count := range fileCommands {
			adjustedCount := count * target.Priority // Weight by file importance

			if existing, exists := commands[cmd]; exists {
				existing.Count += adjustedCount
			} else {
				commands[cmd] = &CommandFrequency{
					Command:  cmd,
					Count:    adjustedCount,
					Sources:  []string{fmt.Sprintf("%s:%s", repo.FullName, target.Path)},
					Category: categorizeCommand(cmd),
				}
			}
		}

		stats.FilesProcessed++
		fileType := getFileType(target.Path)
		stats.FileTypeStats[fileType]++
	}

	return commands, nil
}

func (g *GitHubExtractor) getFileContent(repoName, filePath string) ([]byte, error) {
	cacheKey := fmt.Sprintf("file_%s_%s", repoName, strings.ReplaceAll(filePath, "/", "_"))

	// Check cache
	if !skipCache {
		if cached, err := g.loadFromCache(cacheKey); err == nil {
			return cached, nil
		}
	}

	url := fmt.Sprintf("%s/repos/%s/contents/%s", g.BaseURL, repoName, filePath)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "token "+g.Token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "ghosttype-command-extractor")

	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("file not found or error: %d", resp.StatusCode)
	}

	var fileInfo FileContent
	if err := json.NewDecoder(resp.Body).Decode(&fileInfo); err != nil {
		return nil, err
	}

	// Decode base64 content
	content, err := base64.StdEncoding.DecodeString(fileInfo.Content)
	if err != nil {
		return nil, err
	}

	// Cache the content
	g.saveToCache(cacheKey, content)

	return content, nil
}

// Enhanced parsers
func parseMakefileCommands(content, filePath string) map[string]int {
	commands := make(map[string]int)

	// Match Makefile command patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^\t+([a-zA-Z].+?)(?:\s*\\)?$`),           // Tab-indented commands
		regexp.MustCompile(`^\t+@([a-zA-Z].+?)(?:\s*\\)?$`),          // Silent commands
		regexp.MustCompile(`^\t+\$\(([A-Z_]+)\)\s+(.+?)(?:\s*\\)?$`), // Variable commands
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()

		for _, pattern := range patterns {
			if matches := pattern.FindStringSubmatch(line); len(matches) > 1 {
				cmd := strings.TrimSpace(matches[1])
				if len(matches) > 2 && matches[2] != "" {
					cmd = strings.TrimSpace(matches[2])
				}

				if isValidCommand(cmd) {
					commands[normalizeCommand(cmd)]++
				}
			}
		}
	}

	return commands
}

func parsePackageJsonCommands(content, filePath string) map[string]int {
	commands := make(map[string]int)

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}

	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return commands
	}

	for _, script := range pkg.Scripts {
		// Handle complex script patterns
		parts := strings.Split(script, "&&")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if isValidCommand(part) {
				commands[normalizeCommand(part)]++
			}
		}
	}

	return commands
}

func parseDockerfileCommands(content, filePath string) map[string]int {
	commands := make(map[string]int)

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^RUN\s+(.+)$`),
		regexp.MustCompile(`^ENTRYPOINT\s+\[?"(.+?)"?\]?$`),
		regexp.MustCompile(`^CMD\s+\[?"(.+?)"?\]?$`),
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		for _, pattern := range patterns {
			if matches := pattern.FindStringSubmatch(line); len(matches) > 1 {
				cmd := strings.TrimSpace(matches[1])

				// Handle multi-command lines
				parts := strings.Split(cmd, "&&")
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if isValidCommand(part) {
						commands[normalizeCommand(part)]++
					}
				}
			}
		}
	}

	return commands
}

func parseDockerComposeCommands(content, filePath string) map[string]int {
	commands := make(map[string]int)

	// Simple regex for docker-compose command patterns
	commandRegex := regexp.MustCompile(`command:\s*(.+)`)

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if matches := commandRegex.FindStringSubmatch(line); len(matches) > 1 {
			cmd := strings.Trim(matches[1], `"'`)
			if isValidCommand(cmd) {
				commands[normalizeCommand(cmd)]++
			}
		}
	}

	return commands
}

func parseGitHubActionsCommands(content, filePath string) map[string]int {
	commands := make(map[string]int)

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`run:\s*\|?\s*(.+)`),
		regexp.MustCompile(`run:\s*"(.+)"`),
		regexp.MustCompile(`run:\s*'(.+)'`),
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)

		for _, pattern := range patterns {
			if matches := pattern.FindStringSubmatch(line); len(matches) > 1 {
				cmd := matches[1]

				// Handle multi-line commands
				if strings.Contains(cmd, "|") || (i+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i+1]), " ")) {
					// Multi-line command, collect subsequent lines
					j := i + 1
					for j < len(lines) && strings.HasPrefix(lines[j], "        ") {
						cmd += "\n" + strings.TrimSpace(lines[j])
						j++
					}
				}

				// Parse individual commands
				cmdLines := strings.Split(cmd, "\n")
				for _, cmdLine := range cmdLines {
					cmdLine = strings.TrimSpace(cmdLine)
					if isValidCommand(cmdLine) {
						commands[normalizeCommand(cmdLine)]++
					}
				}
			}
		}
	}

	return commands
}

func parseShellCommands(content, filePath string) map[string]int {
	commands := make(map[string]int)

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments, empty lines, and variable assignments
		if strings.HasPrefix(line, "#") || line == "" ||
			strings.Contains(line, "=") && !strings.Contains(line, " ") {
			continue
		}

		if isValidCommand(line) {
			commands[normalizeCommand(line)]++
		}
	}

	return commands
}

func parseReadmeCommands(content, filePath string) map[string]int {
	commands := make(map[string]int)

	// Enhanced regex for different code block types
	patterns := []*regexp.Regexp{
		regexp.MustCompile("```(?:bash|sh|shell|console|terminal)\n([^`]+)```"),
		regexp.MustCompile("```\n([^`]*(?:npm|yarn|go|cargo|make|docker)[^`]*)```"),
		regexp.MustCompile("`([^`]*(?:npm|yarn|go|cargo|make|docker)[^`]*)`"),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				lines := strings.Split(match[1], "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					line = strings.TrimPrefix(line, "$ ")
					line = strings.TrimPrefix(line, "> ")

					if isValidCommand(line) {
						commands[normalizeCommand(line)]++
					}
				}
			}
		}
	}

	return commands
}

func isValidCommand(cmd string) bool {
	cmd = strings.TrimSpace(cmd)

	// Basic filters
	if len(cmd) < 3 || len(cmd) > 200 {
		return false
	}

	// Skip obvious non-commands
	skipPatterns := []string{
		"echo", "printf", "cat <<", "export ", "source ", ". ",
		"if ", "while ", "for ", "function ", "#!/",
		"//", "/*", "*/",
	}

	cmdLower := strings.ToLower(cmd)
	for _, skip := range skipPatterns {
		if strings.HasPrefix(cmdLower, skip) {
			return false
		}
	}

	// Must match common command patterns
	validPrefixes := []string{
		"npm", "yarn", "pnpm", "bun",
		"go", "cargo", "mvn", "gradle", "make", "cmake",
		"docker", "kubectl", "helm", "podman",
		"git", "svn", "hg",
		"python", "python3", "pip", "pipenv", "poetry",
		"node", "deno", "java", "javac",
		"terraform", "ansible", "vagrant",
		"curl", "wget", "ssh", "scp",
		"ls", "cd", "mkdir", "rm", "cp", "mv", "find", "grep",
		"apt", "yum", "brew", "pacman",
	}

	for _, prefix := range validPrefixes {
		if strings.HasPrefix(cmdLower, prefix+" ") || cmdLower == prefix {
			return true
		}
	}

	return false
}

func normalizeCommand(cmd string) string {
	// Normalize common variations
	cmd = strings.TrimSpace(cmd)

	// Remove common prefixes that add noise
	cmd = strings.TrimPrefix(cmd, "sudo ")
	cmd = strings.TrimPrefix(cmd, "time ")

	// Normalize paths
	cmd = regexp.MustCompile(`/[a-zA-Z0-9/_.-]+`).ReplaceAllString(cmd, "/path")

	// Normalize URLs
	cmd = regexp.MustCompile(`https?://[^\s]+`).ReplaceAllString(cmd, "URL")

	return cmd
}

func categorizeCommand(cmd string) string {
	cmd = strings.ToLower(cmd)

	categories := map[string][]string{
		"package_management": {"npm", "yarn", "pip", "cargo", "go mod", "mvn", "gradle"},
		"containerization":   {"docker", "kubectl", "helm", "podman"},
		"version_control":    {"git", "svn", "hg"},
		"build_tools":        {"make", "cmake", "ninja", "bazel"},
		"infrastructure":     {"terraform", "ansible", "vagrant", "ssh"},
		"file_operations":    {"ls", "cd", "mkdir", "rm", "cp", "mv", "find", "grep"},
		"network":            {"curl", "wget", "ping", "netstat"},
		"system":             {"sudo", "chmod", "chown", "ps", "kill"},
	}

	for category, commands := range categories {
		for _, c := range commands {
			if strings.HasPrefix(cmd, c+" ") || cmd == c {
				return category
			}
		}
	}

	return "other"
}

func getFileType(path string) string {
	if strings.Contains(path, "Makefile") {
		return "makefile"
	} else if strings.Contains(path, "package.json") {
		return "package_json"
	} else if strings.Contains(path, "Dockerfile") {
		return "dockerfile"
	} else if strings.Contains(path, "docker-compose") {
		return "docker_compose"
	} else if strings.Contains(path, ".yml") || strings.Contains(path, ".yaml") {
		return "yaml"
	} else if strings.Contains(path, ".sh") {
		return "shell"
	} else if strings.Contains(path, "README") {
		return "readme"
	}
	return "other"
}

// Cache management
func (g *GitHubExtractor) loadFromCache(key string) ([]byte, error) {
	if data, exists := g.Cache[key]; exists {
		return data, nil
	}

	cachePath := filepath.Join(g.CacheDir, key+".json")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	g.Cache[key] = data
	return data, nil
}

func (g *GitHubExtractor) saveToCache(key string, data []byte) {
	g.Cache[key] = data

	cachePath := filepath.Join(g.CacheDir, key+".json")
	os.WriteFile(cachePath, data, 0644) // Ignore errors for cache
}

// Output functions
func saveExtractionResults(commands []CommandFrequency, stats *ExtractionStats, filename string) error {
	result := struct {
		Commands []CommandFrequency `json:"commands"`
		Stats    *ExtractionStats   `json:"stats"`
		Meta     map[string]string  `json:"meta"`
	}{
		Commands: commands,
		Stats:    stats,
		Meta: map[string]string{
			"generated_at": time.Now().Format(time.RFC3339),
			"tool":         "ghosttype-github-extractor",
			"version":      "1.0",
		},
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func printExtractionSummary(commands []CommandFrequency, stats *ExtractionStats) {
	fmt.Printf("\n🎉 EXTRACTION COMPLETE\n")
	fmt.Printf("═══════════════════════════════════════\n")
	fmt.Printf("⏱️  Processing time: %v\n", stats.ProcessingTime.Round(time.Second))
	fmt.Printf("📦 Repositories processed: %d\n", stats.ReposProcessed)
	fmt.Printf("📄 Files processed: %d\n", stats.FilesProcessed)
	fmt.Printf("🔧 Unique commands found: %d\n", stats.CommandsFound)
	fmt.Printf("⚠️  Errors encountered: %d\n", stats.ErrorCount)

	if len(stats.LanguageStats) > 0 {
		fmt.Printf("\n📊 By Language:\n")
		for lang, count := range stats.LanguageStats {
			fmt.Printf("  %-12s: %d repos\n", lang, count)
		}
	}

	if len(stats.FileTypeStats) > 0 {
		fmt.Printf("\n📁 By File Type:\n")
		for fileType, count := range stats.FileTypeStats {
			fmt.Printf("  %-15s: %d files\n", fileType, count)
		}
	}

	fmt.Printf("\n🏆 Top 20 Commands:\n")
	for i, cmd := range commands[:min(20, len(commands))] {
		fmt.Printf("%2d. %-35s (%3d repos, %.2f%%)\n",
			i+1, truncate(cmd.Command, 35), cmd.Count, cmd.Frequency)
	}

	fmt.Printf("\n💾 Results saved to: %s\n", githubOutput)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// func min(a, b int) int {
// 	if a < b {
// 		return a
// 	}
// 	return b
// }
