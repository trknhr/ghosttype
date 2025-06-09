package prefix

import (
	"database/sql"

	"github.com/trknhr/ghosttype/model"

	_ "github.com/tursodatabase/go-libsql"
)

type PrefixModel struct {
	Counts map[string]int
	Table  string

	db *sql.DB
}

func NewPrefixModel(db *sql.DB) model.SuggestModel {
	return &PrefixModel{Counts: make(map[string]int), db: db, Table: "history"}
}

func (m *PrefixModel) Learn(entries []string) error {
	return nil
}

func (m *PrefixModel) Predict(input string) ([]model.Suggestion, error) {
	query := input + "%"

	rows, err := m.db.Query(`
		SELECT command, count
		FROM  history
		WHERE command LIKE ?
		ORDER BY count DESC
		LIMIT 20;
	`, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.Suggestion
	for rows.Next() {
		var cmd string
		var count int
		if err := rows.Scan(&cmd, &count); err != nil {
			continue
		}
		results = append(results, model.Suggestion{
			Text:   cmd,
			Score:  float64(count),
			Source: "prefix",
		})
	}
	return results, nil
}

func (m *PrefixModel) Weight() float64 {
	return 0.8
}
