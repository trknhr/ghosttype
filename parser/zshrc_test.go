package parser

import (
	"os"
	"testing"
)

func TestExtractZshAliases(t *testing.T) {
	content := `
# comment line
alias ll='ls -alF'
alias gs="git status"
export PATH=$PATH:/usr/local/bin
alias grep='grep --color=auto'
`
	tmpFile, err := os.CreateTemp("", "zshrc_test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name()) // Delete finally
	_, _ = tmpFile.WriteString(content)
	tmpFile.Close()

	aliases, err := ExtractZshAliases(tmpFile.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []Alias{
		{Name: "ll", Cmd: "ls -alF"},
		{Name: "gs", Cmd: "git status"},
		{Name: "grep", Cmd: "grep --color=auto"},
	}

	if len(aliases) != len(expected) {
		t.Errorf("expected %d aliases, got %d", len(expected), len(aliases))
	}

	for i, alias := range expected {
		if aliases[i] != alias {
			t.Errorf("expected alias %v, got %v", alias, aliases[i])
		}
	}
}

func TestExtractZshAliases_FileNotFound(t *testing.T) {
	_, err := ExtractZshAliases("non_existent_file")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}
