package embedding

import (
	"database/sql"
	"errors"

	"github.com/trknhr/ghosttype/internal/logger.go"
	"github.com/trknhr/ghosttype/model"
)

type EmbeddingModel struct {
	weight float64
	store  *EmbeddingStore
}

func NewModel(db *sql.DB, weight float64) model.SuggestModel {
	return &EmbeddingModel{
		weight: weight,
		store:  &EmbeddingStore{DB: db},
	}
}

func (m *EmbeddingModel) Learn(entries []string) error {
	var allErr error

	for _, entry := range entries {
		//  skip
		if m.store.Exists("history", entry) {
			continue
		}

		vec := embedViaOllama(entry)
		err := m.store.Save("history", entry, vec)
		if err != nil {
			logger.Debug("failed to save embedding: %v", err)
			allErr = errors.Join(allErr, err)

		}
	}
	return allErr
}

func (m *EmbeddingModel) Predict(input string) ([]model.Suggestion, error) {
	queryVec := embedViaOllama(input)

	suggestions, err := m.store.SearchSimilar(queryVec, "history", 10, 0.3)
	if err != nil {
		return nil, err
	}

	weighted := make([]model.Suggestion, 0, len(suggestions))
	for _, s := range suggestions {
		s.Score *= m.Weight()
		weighted = append(weighted, s)
	}

	return weighted, nil
}

func (m *EmbeddingModel) Weight() float64 {
	return m.weight
}
