package store_test

import (
	"database/sql"
	"testing"

	_ "github.com/tursodatabase/go-libsql"

	"github.com/trknhr/ghosttype/internal/store"
)

func setupMigrateTestDB(t *testing.T) (*sql.DB, func()) {
	db, err := sql.Open("libsql", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	return db, func() {
		db.Close()
	}
}

func TestMigrate(t *testing.T) {
	db, cleanup := setupMigrateTestDB(t)
	defer cleanup()

	if err := store.Migrate(db); err != nil {
		t.Fatalf("initial migrate failed: %v", err)
	}

	if err := store.Migrate(db); err != nil {
		t.Fatalf("second migrate failed: %v", err)
	}

	expectedTables := []string{
		"history", "history_fts", "aliases", "meta", "embeddings",
	}

	for _, table := range expectedTables {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Errorf("expected table %s to exist: %v", table, err)
		} else if name != table {
			t.Errorf("expected table name %s, got %s", table, name)
		}
	}

	var trigger string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='trigger' AND name='history_ai'`).Scan(&trigger)
	if err != nil {
		t.Errorf("expected trigger history_ai to exist: %v", err)
	} else if trigger != "history_ai" {
		t.Errorf("unexpected trigger name: got %s", trigger)
	}
}
