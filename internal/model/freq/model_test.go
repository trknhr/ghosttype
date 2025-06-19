package freq_test

import (
	"database/sql"
	"testing"

	"github.com/trknhr/ghosttype/internal/model/freq"
	_ "github.com/tursodatabase/go-libsql"
)

func setupFreqTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("libsql", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	// 通常のテーブルと FTS5 仮想テーブル
	_, err = db.Exec(`
		CREATE TABLE history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			command TEXT NOT NULL,
			count INTEGER DEFAULT 1
		);
	`)
	if err != nil {
		t.Fatalf("failed to create history table: %v", err)
	}

	_, err = db.Exec(`
		CREATE VIRTUAL TABLE history_fts USING fts5(command, content='history', content_rowid='id');
	`)
	if err != nil {
		t.Fatalf("failed to create history_fts table: %v", err)
	}

	// データを追加
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
			t.Fatalf("failed to insert into history: %v", err)
		}
		_, err = db.Exec(`INSERT INTO history_fts (rowid, command) VALUES (last_insert_rowid(), ?)`, c.cmd)
		if err != nil {
			t.Fatalf("failed to insert into history_fts: %v", err)
		}
	}

	return db
}

func TestFreqModel_Predict(t *testing.T) {
	db := setupFreqTestDB(t)
	model := freq.NewFreqModel(db)

	results, err := model.Predict("git")
	if err != nil {
		t.Fatalf("Predict failed: %v", err)
	}

	if len(results) < 2 {
		t.Errorf("expected at least 2 results for 'git', got %d", len(results))
	}

	if results[0].Text != "git commit" || results[0].Score != 5 {
		t.Errorf("unexpected top result: %+v", results[0])
	}

	if results[1].Text != "git checkout" || results[1].Score != 3 {
		t.Errorf("unexpected second result: %+v", results[1])
	}
}
