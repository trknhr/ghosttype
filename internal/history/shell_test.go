package history_test

import (
	"os"
	"testing"

	"github.com/trknhr/ghosttype/internal/history"
)

func TestDetectShell(t *testing.T) {
	tests := []struct {
		name     string
		shellEnv string
		want     string
	}{
		{"Zsh", "/bin/zsh", "zsh"},
		{"Bash", "/usr/bin/bash", "bash"},
		{"Fish", "/usr/local/bin/fish", "fish"},
		{"PowerShell", "/usr/bin/pwsh", "powershell"},
		{"PowerShell full", "/usr/bin/powershell", "powershell"},
		{"Unknown", "/bin/foobar", "unknown"},
		{"Empty", "", "unknown"},
	}

	original := os.Getenv("SHELL") // 元の値を保存
	defer os.Setenv("SHELL", original)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("SHELL", tt.shellEnv)
			if got := history.DetectShell(); got != tt.want {
				t.Errorf("DetectShell() = %v, want %v", got, tt.want)
			}
		})
	}
}
