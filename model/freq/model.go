package freq

import (
	"database/sql"

	"github.com/trknhr/ghosttype/model"

	_ "github.com/tursodatabase/go-libsql"
)

type FreqModel struct {
	Counts map[string]int
	Table  string

	db *sql.DB
}

func NewFreqModel(db *sql.DB) model.SuggestModel {
	return &FreqModel{Counts: make(map[string]int), db: db, Table: "history"}
}

func (m *FreqModel) Learn(entries []string) error {
	return nil
}

func (m *FreqModel) Predict(input string) ([]model.Suggestion, error) {
	rows, err := m.db.Query(`
		SELECT h.command, count
		FROM history_fts f
		JOIN history h ON f.rowid = h.id
		WHERE f.command MATCH ? || '*'
		ORDER BY h.count DESC
		LIMIT 20;
	`, input)
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
			Source: "freq",
		})
	}
	return results, nil
}

func (m *FreqModel) Weight() float64 {
	return 0.5
}
