package history

import (
	"bufio"
	"os"
	"strings"
)

// LoadZshHistory reads a .zsh_history file and extracts command lines, ignoring timestamps.
func LoadZshHistory(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var commands []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// zsh history lines often look like: ": 1683776572:0;git status"
		if strings.HasPrefix(line, ": ") {
			if parts := strings.SplitN(line, ";", 2); len(parts) == 2 {
				commands = append(commands, strings.TrimSpace(parts[1]))
			}
		} else {
			// fallback for nonstandard lines
			commands = append(commands, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return commands, nil
}
