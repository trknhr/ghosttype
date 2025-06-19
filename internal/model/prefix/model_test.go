package prefix_test

import (
	"database/sql"
	"testing"

	"github.com/trknhr/ghosttype/internal/model/prefix"
	_ "github.com/tursodatabase/go-libsql"
)

func setupPrefixTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("libsql", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			command TEXT NOT NULL,
			count INTEGER DEFAULT 1
		);
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// insert sample data
	commands := []struct {
		cmd   string
		count int
	}{
		{"git commit", 5},
		{"git checkout", 3},
		{"go build", 2},
		{"npm install", 1},
	}
	for _, c := range commands {
		_, err := db.Exec(`INSERT INTO history (command, count) VALUES (?, ?)`, c.cmd, c.count)
		if err != nil {
			t.Fatalf("failed to insert command %q: %v", c.cmd, err)
		}
	}

	return db
}

func TestPrefixModel_Predict(t *testing.T) {
	db := setupPrefixTestDB(t)
	model := prefix.NewPrefixModel(db)

	results, err := model.Predict("git")
	if err != nil {
		t.Fatalf("Predict failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results for prefix 'git', got %d", len(results))
	}

	if results[0].Text != "git commit" || results[0].Score != 5 {
		t.Errorf("expected first result to be 'git commit' with score 5, got %+v", results[0])
	}

	if results[1].Text != "git checkout" || results[1].Score != 3 {
		t.Errorf("expected second result to be 'git checkout' with score 3, got %+v", results[1])
	}
}
