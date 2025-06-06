package internal

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	_ "github.com/tursodatabase/go-libsql"
)

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
