package store

import (
	"database/sql"
	"fmt"
	"os"
)

type MetaStore struct {
	db *sql.DB
}

func NewMetaStore(db *sql.DB) *MetaStore {
	return &MetaStore{db: db}
}
func (m *MetaStore) TouchMeta(key string, filePath string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("stat error for %s: %w", filePath, err)
	}

	mtime := info.ModTime().Unix()

	_, err = m.db.Exec(`
		INSERT OR REPLACE INTO meta (key, path, mtime)
		VALUES (?, ?, ?)
	`, key, filePath, mtime)

	if err != nil {
		return fmt.Errorf("failed to update meta: %w", err)
	}

	return nil
}

func (m *MetaStore) NeedsReload(key string, filePath string) bool {
	info, err := os.Stat(filePath)
	if err != nil {
		return true // If there is no file, it'll reload
	}
	currentMtime := info.ModTime().Unix()

	var storedMtime int64
	err = m.db.QueryRow(`SELECT mtime FROM meta WHERE key = ?`, key).Scan(&storedMtime)
	if err != nil {
		return true
	}

	return currentMtime > storedMtime
}
