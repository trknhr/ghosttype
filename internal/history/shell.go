package history

import (
	"os"
	"path/filepath"
	"strings"
)

// DetectShell returns the detected shell type (e.g. "zsh", "bash", "fish", "unknown")
func DetectShell() string {
	shellPath := os.Getenv("SHELL")
	base := filepath.Base(shellPath)
	switch {
	case strings.Contains(base, "zsh"):
		return "zsh"
	case strings.Contains(base, "bash"):
		return "bash"
	case strings.Contains(base, "fish"):
		return "fish"
	case strings.Contains(base, "pwsh"), strings.Contains(base, "powershell"):
		return "powershell"
	default:
		return "unknown"
	}
}
