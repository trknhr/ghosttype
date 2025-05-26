package alias_test

import (
	"testing"

	"github.com/trknhr/ghosttype/model/alias"
)

func TestAliasModel_Predict(t *testing.T) {
	mockStore := &MockAliasStore{
		Results: []alias.AliasEntry{
			{Name: "gs", Cmd: "git status"},
			{Name: "ll", Cmd: "ls -alF"},
		},
	}

	model := alias.NewAliasModel(mockStore)

	results, err := model.Predict("g")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

type MockAliasStore struct {
	Results []alias.AliasEntry
	Err     error
}

func (m *MockAliasStore) QueryAliases(input string) ([]alias.AliasEntry, error) {
	return m.Results, m.Err
}
