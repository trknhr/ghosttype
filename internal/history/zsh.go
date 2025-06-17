package history

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

// LoadZshHistoryCommands loads full zsh history commands from file.
// It supports:
// - line continuation with '\'
// - EXTENDED_HISTORY format
// - ending continuation on empty line
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
		trimmed := strings.TrimSpace(line)

		// Case 1: empty line ends current command
		if trimmed == "" {
			if current.Len() > 0 {
				commands = append(commands, strings.TrimSpace(current.String()))
				current.Reset()
			}
			continue
		}

		// Case 2: EXTENDED_HISTORY prefix line
		if strings.HasPrefix(line, ": ") && strings.Contains(line, ";") {
			if current.Len() > 0 {
				commands = append(commands, strings.TrimSpace(current.String()))
				current.Reset()
			}
			if idx := strings.Index(line, ";"); idx != -1 && idx+1 < len(line) {
				line = line[idx+1:]
			}
		}

		// Case 3: line continuation
		if strings.HasSuffix(line, "\\") {
			current.WriteString(strings.TrimSpace(strings.TrimSuffix(line, "\\")))
			current.WriteString(" ")
			continue
		}

		// Case 4: regular line, flush after writing
		current.WriteString(line)
		commands = append(commands, strings.TrimSpace(current.String()))
		current.Reset()
	}

	// Final flush if needed
	if current.Len() > 0 {
		commands = append(commands, strings.TrimSpace(current.String()))
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return commands, nil
}

func LoadFilteredZshHistory(path string) ([]string, error) {
	cmds, err := LoadZshHistoryCommands(path)
	if err != nil {
		return nil, err
	}

	var filtered []string
	for _, cmd := range cmds {
		if isValidCommand(cmd) {
			filtered = append(filtered, cmd)
		}
	}
	return filtered, nil
}

func isValidCommand(line string) bool {
	trimmed := strings.TrimSpace(line)

	// Skip empty or very short lines
	if len(trimmed) < 3 {
		return false
	}

	// Skip overlong lines (probably pasted accidentally)
	if len(trimmed) > 500 {
		return false
	}

	// Skip lines with only one token that's a symbol or flag
	fields := strings.Fields(trimmed)
	if len(fields) == 1 {
		first := fields[0]
		if strings.HasPrefix(first, "-") || strings.HasPrefix(first, "--") {
			return false
		}
		if strings.Count(first, "/") > 2 || strings.Contains(first, "=") {
			return false // e.g. paths or key=value pairs alone
		}
	}

	// Skip JSON fragments or likely malformed input
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return false
	}
	if strings.HasSuffix(trimmed, ":") {
		return false
	}

	// Passed all heuristics
	return true
}

func LoadZshHistoryTail(path string, maxLines int) ([]string, error) {
	const readBlockSize = 4096

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}

	var (
		fileSize = fi.Size()
		buf      []byte
		lines    []string
	)

	var offset int64 = fileSize
	var leftover []byte

	for offset > 0 && len(lines) < maxLines {
		// Move back by readBlockSize or to the beginning
		blockSize := int64(readBlockSize)
		if offset < blockSize {
			blockSize = offset
		}
		offset -= blockSize

		// Seek and read
		block := make([]byte, blockSize)
		_, err := file.ReadAt(block, offset)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("read error: %w", err)
		}

		buf = append(block, leftover...)
		scanner := bufio.NewScanner(bytes.NewReader(buf))
		var blockLines []string
		for scanner.Scan() {
			blockLines = append(blockLines, scanner.Text())
		}
		leftover = []byte(blockLines[0])
		for i := len(blockLines) - 1; i > 0; i-- {
			line := cleanZshLine(blockLines[i])
			if line != "" {
				lines = append(lines, line)
			}
			if len(lines) >= maxLines {
				break
			}
		}
	}

	// Reverse to restore chronological order
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}

	return lines, nil
}

func cleanZshLine(line string) string {
	line = string(bytes.TrimSpace([]byte(line)))
	if strings.HasPrefix(line, ": ") {
		parts := strings.SplitN(line, ";", 2)
		if len(parts) == 2 {
			return string(strings.TrimSpace(parts[1]))
		}
		return ""
	}
	return string(line)
}
