package embedding_test

import (
	"testing"

	"github.com/trknhr/ghosttype/internal/model/embedding"
	"github.com/trknhr/ghosttype/internal/model/entity"
	"github.com/trknhr/ghosttype/internal/ollama"
)

type MockClient struct {
	EmbedFunc    func(text string) (*ollama.OllamaEmbedResponse, error)
	GenerateFunc func(prompt string) (*ollama.OllamaCompleteResponse, error)
}

func (m *MockClient) Embed(text string) (*ollama.OllamaEmbedResponse, error) {
	if m.EmbedFunc != nil {
		return m.EmbedFunc(text)
	}
	return nil, nil
}

func (m *MockClient) Generate(prompt string) (*ollama.OllamaCompleteResponse, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(prompt)
	}
	return nil, nil
}

type MockStore struct {
	ExistsFunc        func(string, string) bool
	SaveFunc          func(string, string, []float32) error
	SearchSimilarFunc func([]float32, string, int, float64) ([]entity.Suggestion, error)
}

func (m *MockStore) Exists(source, text string) bool {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(source, text)
	}
	return false
}

func (m *MockStore) Save(source, text string, vec []float32) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(source, text, vec)
	}
	return nil
}

func (m *MockStore) SearchSimilar(vec []float32, source string, topK int, threshold float64) ([]entity.Suggestion, error) {
	if m.SearchSimilarFunc != nil {
		return m.SearchSimilarFunc(vec, source, topK, threshold)
	}
	return nil, nil
}

func TestEmbeddingModel_LearnAndPredict_WithMocks(t *testing.T) {
	embedCalled := 0
	saveCalled := 0
	predictCalled := 0

	mockClient := &MockClient{
		EmbedFunc: func(text string) (*ollama.OllamaEmbedResponse, error) {
			embedCalled++
			return &ollama.OllamaEmbedResponse{Embedding: []float32{0.1, 0.2, 0.3}}, nil
		},
	}

	mockStore := &MockStore{
		ExistsFunc: func(source, text string) bool {
			return false
		},
		SaveFunc: func(source, text string, vec []float32) error {
			saveCalled++
			return nil
		},
		SearchSimilarFunc: func(vec []float32, source string, topK int, threshold float64) ([]entity.Suggestion, error) {
			predictCalled++
			return []entity.Suggestion{
				{Text: "git push", Score: 0.9, Source: source},
			}, nil
		},
	}

	model := embedding.NewModel(mockStore, mockClient)

	err := model.Learn([]string{"git push", "npm install"})
	if err != nil {
		t.Fatalf("Learn failed: %v", err)
	}

	if saveCalled != 2 {
		t.Errorf("expected Save to be called 2 times, got %d", saveCalled)
	}

	results, err := model.Predict("git")
	if err != nil {
		t.Fatalf("Predict failed: %v", err)
	}

	if len(results) != 1 || results[0].Text != "git push" {
		t.Errorf("unexpected prediction result: %+v", results)
	}

	if embedCalled != 3 {
		t.Errorf("expected Embed to be called 3 times, got %d", embedCalled)
	}
	if predictCalled != 1 {
		t.Errorf("expected SearchSimilar to be called once")
	}
}
