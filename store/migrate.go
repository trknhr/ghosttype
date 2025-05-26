package store

import (
	"database/sql"
	"fmt"

	_ "github.com/tursodatabase/go-libsql"
)

func Migrate(db *sql.DB) error {
	schema := []string{
		// aliases: ALIAS commands on zshrc
		`CREATE TABLE IF NOT EXISTS aliases (
			name TEXT PRIMARY KEY,
			cmd TEXT NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		// meta: Latest updated time of .zshrc etc
		`CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			path TEXT NOT NULL,
			mtime INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS embeddings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source TEXT NOT NULL,
			text TEXT NOT NULL,
			emb F32_BLOB(768),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS embeddings_idx ON embeddings(libsql_vector_idx(emb));`,
	}

	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("failed to run migration statement: %w", err)
		}
	}

	return nil
}
