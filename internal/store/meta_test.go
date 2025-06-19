package store_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/trknhr/ghosttype/internal/store"
	_ "github.com/tursodatabase/go-libsql"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	db, err := sql.Open("libsql", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE meta (key TEXT PRIMARY KEY, path TEXT, mtime INTEGER)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	return db, func() {
		db.Close()
	}
}

func TestMetaStore_TouchMetaAndNeedsReload(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	meta := store.NewMetaStore(db)

	// Create a temp file
	tmpfile := filepath.Join(t.TempDir(), "sample.txt")
	err := os.WriteFile(tmpfile, []byte("hello"), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	key := "testfile"

	// First call should update the meta
	err = meta.TouchMeta(key, tmpfile)
	if err != nil {
		t.Fatalf("TouchMeta failed: %v", err)
	}

	// Should not need reload immediately after
	if meta.NeedsReload(key, tmpfile) {
		t.Fatalf("expected no reload, but got reload")
	}

	// Simulate file update
	time.Sleep(1 * time.Second)
	err = os.WriteFile(tmpfile, []byte("new content"), 0644)
	if err != nil {
		t.Fatalf("failed to update file: %v", err)
	}

	if !meta.NeedsReload(key, tmpfile) {
		t.Fatalf("expected reload, but got no reload")
	}
}
