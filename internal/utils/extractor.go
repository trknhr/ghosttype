package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ExtractMakeTargets(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var commands []string
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		// target: ...（ただし .PHONY や .SILENT を除く）
		if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			target := strings.TrimSpace(line[:idx])
			if target != "" && !strings.HasPrefix(target, ".") {
				commands = append(commands, fmt.Sprintf("make %s", target))
			}
		}
	}
	return commands, nil
}

func ExtractMavenTargets(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// minimal heuristic (real parsing via encoding/xml also可)
	candidates := []string{"clean", "validate", "compile", "test", "package", "verify", "install", "site", "deploy"}
	var cmds []string
	for _, phase := range candidates {
		if strings.Contains(string(content), "<phase>"+phase+"</phase>") ||
			strings.Contains(string(content), "<goal>"+phase+"</goal>") {
			cmds = append(cmds, "mvn "+phase)
		}
	}
	return cmds, nil
}

func ExtractNpmScripts(pkgJsonPath string) ([]string, error) {
	absPath, err := filepath.Abs(pkgJsonPath)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read package.json: %w", err)
	}

	var data struct {
		Scripts map[string]string `json:"scripts"`
	}

	if err := json.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	var commands []string
	for name := range data.Scripts {
		commands = append(commands, fmt.Sprintf("npm run %s", name))
	}

	return commands, nil
}
