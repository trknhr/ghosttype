package embedding

import (
	"errors"

	"github.com/trknhr/ghosttype/internal/logger.go"
	"github.com/trknhr/ghosttype/model"
	"github.com/trknhr/ghosttype/ollama"
)

type EmbeddingModel struct {
	store  EmbeddingStore
	client ollama.OllamaClient
}

func NewModel(store EmbeddingStore, client ollama.OllamaClient) model.SuggestModel {
	return &EmbeddingModel{
		store:  store,
		client: client,
	}
}

func (m *EmbeddingModel) Learn(entries []string) error {
	var allErr error

	for _, entry := range entries {
		//  skip
		if m.store.Exists("history", entry) {
			continue
		}

		resp, err := m.client.Embed(entry)
		if err != nil {
			logger.Debug("failed to save embedding: %v", err)
			allErr = errors.Join(allErr, err)
			continue
		}

		err = m.store.Save("history", entry, resp.Embedding)
		if err != nil {
			logger.Debug("failed to save embedding: %v", err)
			allErr = errors.Join(allErr, err)
		}
	}
	return allErr
}

func (m *EmbeddingModel) Predict(input string) ([]model.Suggestion, error) {
	resp, err := m.client.Embed(input)
	if err != nil {
		return nil, err
	}

	suggestions, err := m.store.SearchSimilar(resp.Embedding, "history", 10, 0.5)
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
	return 0.6
}
