package alias

import (
	"database/sql"
)

type SQLAliasStore struct {
	DB *sql.DB
}

func NewSQLAliasStore(db *sql.DB) AliasStore {
	return &SQLAliasStore{DB: db}
}

func (s *SQLAliasStore) QueryAliases(input string) ([]AliasEntry, error) {
	query := `
		SELECT name, cmd FROM aliases
		WHERE name LIKE ? OR cmd LIKE ?
		ORDER BY updated_at DESC LIMIT 10
	`
	rows, err := s.DB.Query(query, input+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AliasEntry
	for rows.Next() {
		var name, cmd string
		if err := rows.Scan(&name, &cmd); err != nil {
			continue
		}
		entries = append(entries, AliasEntry{Name: name, Cmd: cmd})
	}
	return entries, nil
}
