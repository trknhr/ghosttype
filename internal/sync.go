package internal

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	"github.com/trknhr/ghosttype/internal/logger.go"
	"github.com/trknhr/ghosttype/parser"
	"github.com/trknhr/ghosttype/store"
)

func SyncAliasesAsync(db *sql.DB) {
	go func() {
		shell := os.Getenv("SHELL")
		var rcPath string

		switch {
		case strings.Contains(shell, "zsh"):
			rcPath = filepath.Join(os.Getenv("HOME"), ".zshrc")
		case strings.Contains(shell, "bash"):
			rcPath = filepath.Join(os.Getenv("HOME"), ".bashrc")
		default:
			logger.Debug("unsupported shell for alias sync")
			return
		}
		meta := store.NewMetaStore(db)

		if !meta.NeedsReload("aliases", rcPath) {
			logger.Debug("alias sync skipped (up-to-date)")
			return
		}

		aliases, err := parser.ExtractZshAliases(rcPath)
		logger.Debug("aliases %v", aliases)
		if err != nil {
			logger.Debug("failed to parse aliases: %v", err)
			return
		}

		for _, a := range aliases {
			_, err = db.Exec(`INSERT OR REPLACE INTO aliases (name, cmd) VALUES (?, ?)`, a.Name, a.Cmd)
			if err != nil {
				logger.Error("failed to register alias: %s, %s", a.Name, a.Cmd)
			}
		}
		meta.TouchMeta("aliases", rcPath)

		logger.Debug("synced %d aliases", len(aliases))
	}()
}
