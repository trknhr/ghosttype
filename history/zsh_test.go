package history

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadZshHistoryCommands(t *testing.T) {
	mockHistory := `
ls -la
echo "hello world"
echo first line \
second line \
third line
git commit -m "initial"
echo done
npm install -y\

docker build
`

	tmpDir := t.TempDir()
	histPath := filepath.Join(tmpDir, ".zsh_history")

	if err := os.WriteFile(histPath, []byte(mockHistory), 0644); err != nil {
		t.Fatalf("failed to write mock history file: %v", err)
	}

	cmds, err := LoadZshHistoryCommands(histPath)
	if err != nil {
		t.Fatalf("LoadZshHistoryCommands failed: %v", err)
	}
	fmt.Println(cmds)

	expected := []string{
		"ls -la",
		`echo "hello world"`,
		"echo first line second line third line",
		`git commit -m "initial"`,
		`echo done`,
		`npm install -y`,
		`docker build`,
	}

	if len(cmds) != len(expected) {
		t.Errorf("expected %d commands, got %d", len(expected), len(cmds))
		for i, cmd := range cmds {
			t.Logf("actual[%d]: %q", i, cmd)
		}
	}

	for i, exp := range expected {
		if i >= len(cmds) {
			break
		}
		if cmds[i] != exp {
			t.Errorf("command[%d] mismatch\n  expected: %q\n       got: %q", i, exp, cmds[i])
		}
	}
}

func TestLoadFilteredZshHistory(t *testing.T) {
	mockHistory := `
ls -la
a
--help
cd
     
{"type":"chat
npm install -D vitest
go get -u github.com/libsql/client-go/libsql
git commit -m "init"
node_modules
`

	tmpDir := t.TempDir()
	histPath := filepath.Join(tmpDir, ".zsh_history")

	if err := os.WriteFile(histPath, []byte(mockHistory), 0644); err != nil {
		t.Fatalf("failed to write mock history file: %v", err)
	}

	cmds, err := LoadFilteredZshHistory(histPath)
	if err != nil {
		t.Fatalf("LoadFilteredZshHistory failed: %v", err)
	}

	expected := []string{
		"ls -la",
		"npm install -D vitest",
		"go get -u github.com/libsql/client-go/libsql",
		`git commit -m "init"`,
		`node_modules`,
	}

	if len(cmds) != len(expected) {
		t.Errorf("expected %d filtered commands, got %d", len(expected), len(cmds))
		t.Logf("actual filtered commands: %#v", cmds)
	}

	for i, exp := range expected {
		if i >= len(cmds) {
			break
		}
		if cmds[i] != exp {
			t.Errorf("command[%d] mismatch\n  expected: %q\n       got: %q", i, exp, cmds[i])
		}
	}
}
