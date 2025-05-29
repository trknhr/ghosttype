package cmd_test

import (
	"database/sql"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/trknhr/ghosttype/cmd"
)

// Run rootCmd without launching TUI (mock Program.Run)
func TestTuiCommand_InitialArgPassesToModel(t *testing.T) {
	// Setup mock DB (in-memory SQLite or nil for now)
	db, _ := sql.Open("sqlite3", ":memory:")

	// Patch os.OpenFile to avoid real /dev/tty access
	origOpenFile := cmd.OpenFileForTTY
	cmd.OpenFileForTTY = func(name string, flag int, perm os.FileMode) (*os.File, error) {
		return os.NewFile(0, os.DevNull), nil
	}
	defer func() { cmd.OpenFileForTTY = origOpenFile }()

	// Replace model.NewProgram to avoid launching real TUI
	// Optionally patch tea.NewProgram.Run to a stub

	// Run CLI command
	// rootCmd := cmd.GetRootCmd()
	rootCmd.SetArgs([]string{"tui", "git ch"})
	err := rootCmd.Execute(db)

	assert.NoError(t, err)
}
