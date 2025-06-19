package alias_test

import (
	"database/sql"
	"testing"

	"github.com/trknhr/ghosttype/internal/model/alias"
	_ "github.com/tursodatabase/go-libsql"
)

func setupAliasTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("libsql", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS aliases (
			name TEXT PRIMARY KEY,
			cmd TEXT NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`)
	if err != nil {
		t.Fatalf("failed to create aliases table: %v", err)
	}

	_, err = db.Exec(`
	INSERT INTO aliases (name, cmd) VALUES
		('gcm', 'git commit'),
		('gst', 'git status'),
		('gaa', 'git add .');
`)

	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	return db
}

func TestSQLAliasStore_QueryAliases(t *testing.T) {
	db := setupAliasTestDB(t)
	store := alias.NewSQLAliasStore(db)

	// partial match
	results, err := store.QueryAliases("g")
	if err != nil {
		t.Fatalf("QueryAliases failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("expected results, got none")
	}

	found := false
	for _, r := range results {
		if r.Cmd == "git commit" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find 'git commit' in results")
	}
}
