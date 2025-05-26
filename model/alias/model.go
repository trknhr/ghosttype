package alias

import (
	"github.com/trknhr/ghosttype/model"
)

type AliasModel struct {
	store AliasStore
}

func NewAliasModel(aliasStore AliasStore) model.SuggestModel {
	return &AliasModel{store: aliasStore}
}

func (m *AliasModel) Learn(entries []string) error {
	return nil
}

func (m *AliasModel) Predict(input string) ([]model.Suggestion, error) {
	entries, err := m.store.QueryAliases(input)
	if err != nil {
		return nil, err
	}

	var results []model.Suggestion
	for _, e := range entries {
		results = append(results, model.Suggestion{
			Text:   e.Name,
			Score:  1.0,
			Source: "alias",
		})
	}
	return results, nil
}

func (m *AliasModel) Weight() float64 {
	return 0.8
}
