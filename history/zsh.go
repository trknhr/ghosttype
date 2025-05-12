package history

import (
	"bufio"
	"os"
	"strings"
)

// LoadZshHistoryCommands loads full commands (including multiline) from ~/.zsh_history
func LoadZshHistoryCommands(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var commands []string
	var current strings.Builder
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip metadata lines
		if strings.HasPrefix(line, ": ") {
			// Start of a new command
			if current.Len() > 0 {
				commands = append(commands, current.String())
				current.Reset()
			}
			idx := strings.Index(line, ";")
			if idx != -1 && idx+1 < len(line) {
				line = line[idx+1:]
			}
		}

		// Handle line continuation (escaped newline)
		if strings.HasSuffix(line, "\\") {
			current.WriteString(strings.TrimSuffix(line, "\\"))
			current.WriteString(" ")
		} else {
			current.WriteString(line)
			commands = append(commands, current.String())
			current.Reset()
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return commands, nil
}
