package worker

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	"github.com/trknhr/ghosttype/internal/logger"
	"github.com/trknhr/ghosttype/internal/parser"
	"github.com/trknhr/ghosttype/internal/store"
)

type AliasSyncWorker struct {
	db     *sql.DB
	rcPath string
	meta   *store.MetaStore
}

func NewAliasSyncWorker(db *sql.DB) *AliasSyncWorker {
	shell := os.Getenv("SHELL")
	var rcPath string
	switch {
	case strings.Contains(shell, "zsh"):
		rcPath = filepath.Join(os.Getenv("HOME"), ".zshrc")
	case strings.Contains(shell, "bash"):
		rcPath = filepath.Join(os.Getenv("HOME"), ".bashrc")
	default:
		logger.Debug("unsupported shell for alias sync")
		rcPath = ""
	}
	return &AliasSyncWorker{
		db:     db,
		rcPath: rcPath,
		meta:   store.NewMetaStore(db),
	}
}

func (a *AliasSyncWorker) Key() string  { return "aliases" }
func (a *AliasSyncWorker) Path() string { return a.rcPath }
func (a *AliasSyncWorker) NeedsReload() bool {
	if a.rcPath == "" {
		return false
	}
	return a.meta.NeedsReload(a.Key(), a.rcPath)
}
func (a *AliasSyncWorker) Sync() error {
	if a.rcPath == "" {
		return nil
	}
	aliases, err := parser.ExtractZshAliases(a.rcPath)
	if err != nil {
		logger.Debug("failed to parse aliases: %v", err)
		return err
	}
	for _, al := range aliases {
		_, err := a.db.Exec(`INSERT OR REPLACE INTO aliases (name, cmd) VALUES (?, ?)`, al.Name, al.Cmd)
		if err != nil {
			logger.Error("failed to register alias: %s, %s", al.Name, al.Cmd)
		}
	}
	a.meta.TouchMeta(a.Key(), a.rcPath)
	logger.Debug("synced %d aliases", len(aliases))
	return nil
}
