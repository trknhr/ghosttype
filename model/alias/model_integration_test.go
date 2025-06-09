package alias_test

import (
	"database/sql"
	"testing"

	"github.com/trknhr/ghosttype/model"
	"github.com/trknhr/ghosttype/model/alias"

	_ "github.com/tursodatabase/go-libsql"
)

func setupInMemoryDB(t *testing.T) *sql.DB {
	db, err := sql.Open("libsql", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE aliases (
			name TEXT PRIMARY KEY,
			cmd TEXT NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatalf("failed to create aliases table: %v", err)
	}

	// データ挿入
	_, err = db.Exec(`
		INSERT INTO aliases (name, cmd) VALUES 
		('gs', 'git status'),
		('gl', 'git log'),
		('ll', 'ls -alF'),
		('ga', 'git add')
	`)
	if err != nil {
		t.Fatalf("failed to insert test aliases: %v", err)
	}

	return db
}

func TestAliasModel_Predict_SQLFiltering(t *testing.T) {
	db := setupInMemoryDB(t)
	defer db.Close()

	model := alias.NewAliasModel(alias.NewSQLAliasStore(db))

	tests := []struct {
		input    string
		expected []string
	}{
		{"g", []string{"gs", "gl", "ga"}},
		{"l", []string{"ll"}},
		{"x", nil},
	}

	for _, tt := range tests {
		results, err := model.Predict(tt.input)
		if err != nil {
			t.Errorf("unexpected error for input %q: %v", tt.input, err)
			continue
		}

		got := extractTexts(results)
		if !equalSlice(got, tt.expected) {
			t.Errorf("Predict(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func extractTexts(suggestions []model.Suggestion) []string {
	var out []string
	for _, s := range suggestions {
		out = append(out, s.Text)
	}
	return out
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]bool{}
	for _, s := range a {
		seen[s] = true
	}
	for _, s := range b {
		if !seen[s] {
			return false
		}
	}
	return true
}
