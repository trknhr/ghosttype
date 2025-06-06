// cmd/learn_history.go
package cmd

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"github.com/trknhr/ghosttype/history"
	"github.com/trknhr/ghosttype/internal"
	"github.com/trknhr/ghosttype/internal/logger.go"
	_ "github.com/tursodatabase/go-libsql"
)

var LearnHistoryCmd = &cobra.Command{
	Use:   "load-history",
	Short: "Background worker to learn full shell history",
	RunE: func(cmd *cobra.Command, args []string) error {
		lockFile := "/tmp/ghosttype-history.lock"
		f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			logger.Debug("learn-history already running")
			return nil
		}
		defer os.Remove(lockFile)
		defer f.Close()

		logger.Debug("started learn-history worker")

		// 学習処理（3分以内に完了）
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		if err := RunHistoryWorker(ctx); err != nil {
			logger.Error("worker failed: %v", err)
		}
		return nil
	},
}

func RunHistoryWorker(ctx context.Context) error {
	historyPath := os.Getenv("HOME") + "/.zsh_history"
	historyEntries, err := history.LoadZshHistoryCommands(historyPath)
	logger.Debug("loaded %d history entries from %s", len(historyEntries), historyPath)
	if err != nil {
		return err
	}

	var cleaned []string
	for _, entry := range historyEntries {
		splits := strings.FieldsFunc(entry, func(r rune) bool {
			return r == ';' || r == '&' || r == '|'
		})
		for _, s := range splits {
			s = strings.TrimSpace(s)
			if s != "" && utf8.ValidString(s) {
				cleaned = append(cleaned, s)
			}
		}
	}

	db := GetDB() // 内部で *sql.DB を取得
	return internal.SaveHistory(db, cleaned)
}

func GetDB() *sql.DB {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Fatalf("failed to get user cache dir: %v", err)
	}
	dbPath := filepath.Join(cacheDir, "ghosttype", "ghosttype.db")

	_ = os.MkdirAll(filepath.Dir(dbPath), 0755)

	db, err := sql.Open("libsql", dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	return db
}
