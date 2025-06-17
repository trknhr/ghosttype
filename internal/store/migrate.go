package store

import (
	"database/sql"
	"fmt"

	_ "github.com/tursodatabase/go-libsql"
)

func Migrate(db *sql.DB) error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS history (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			command     TEXT NOT NULL,
			hash        TEXT NOT NULL UNIQUE,         
			count       INTEGER NOT NULL DEFAULT 1,
			source      TEXT DEFAULT 'shell',
			session_id  TEXT DEFAULT '',
			created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		"CREATE INDEX IF NOT EXISTS idx_history_command_prefix ON history(command);",
		`CREATE VIRTUAL TABLE IF NOT EXISTS history_fts USING fts5(
			command, content='history', content_rowid='id'
		);`,
		`CREATE TRIGGER IF NOT EXISTS history_ai AFTER INSERT ON history BEGIN
			INSERT INTO history_fts(rowid, command) VALUES (new.id, new.command);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS history_ad AFTER DELETE ON history BEGIN
			INSERT INTO history_fts(history_fts, rowid, command) VALUES ('delete', old.id, old.command);
		END;`,
		`CREATE TRIGGER IF NOT EXISTS history_au AFTER UPDATE ON history BEGIN
			INSERT INTO history_fts(history_fts, rowid, command) VALUES ('delete', old.id, old.command);
			INSERT INTO history_fts(rowid, command) VALUES (new.id, new.command);
		END;`,
		`CREATE INDEX IF NOT EXISTS idx_history_hash ON history(hash);`,
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
