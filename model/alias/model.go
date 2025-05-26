package alias

import (
	"database/sql"
	"strings"

	"github.com/trknhr/ghosttype/model"
)

type AliasModel struct {
	db *sql.DB
}

func NewAliasModel(db *sql.DB) model.SuggestModel {
	return &AliasModel{db: db}
}

func (m *AliasModel) Learn(entries []string) error {
	// alias doen't learn
	return nil
}

func (m *AliasModel) Predict(input string) ([]model.Suggestion, error) {
	query := `
		SELECT name, cmd FROM aliases
		WHERE name LIKE ? OR cmd LIKE ?
		ORDER BY updated_at DESC LIMIT 10
	`
	rows, err := m.db.Query(query, input+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.Suggestion
	for rows.Next() {
		var name, cmd string
		if err := rows.Scan(&name, &cmd); err != nil {
			continue
		}

		results = append(results, model.Suggestion{
			Text:   strings.TrimSpace(name),
			Score:  1.0,
			Source: "alias",
		})
	}
	return results, nil
}

func (m *AliasModel) Weight() float64 {
	return 0.8
}
