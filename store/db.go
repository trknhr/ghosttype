package store

import (
	"database/sql"
	"os"
	"path/filepath"

	"github.com/trknhr/ghosttype/internal/logger.go"
)

func OpenDefaultDB() (*sql.DB, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(cacheDir, "ghosttype", "ghosttype.db")
	logger.Debug("dbPath: %s", dbPath)

	os.MkdirAll(filepath.Dir(dbPath), 0755)

	db, err := sql.Open("libsql", "file:"+dbPath)
	if err != nil {
		return nil, err
	}

	return db, nil
}
