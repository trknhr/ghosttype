package llm_test

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/trknhr/ghosttype/internal/model/llm"
	"github.com/trknhr/ghosttype/internal/ollama"
)

func TestLLMRemoteModel_Predict(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := ollama.NewMockOllamaClient(ctrl)

	// Set up expected prompt
	gomock.InOrder(
		mockClient.EXPECT().
			Generate(gomock.Any()).
			Return(&ollama.OllamaCompleteResponse{
				Response: "git status\ngit commit\ngit push",
			}, nil),
	)

	model := llm.NewLLMRemoteModel(mockClient)
	suggestions, err := model.Predict("git")
	if err != nil {
		t.Fatalf("Predict failed: %v", err)
	}

	if len(suggestions) != 3 {
		t.Fatalf("expected 3 suggestions, got %d", len(suggestions))
	}

	expected := []string{"git status", "git commit", "git push"}
	for i, s := range suggestions {
		if s.Text != expected[i] {
			t.Errorf("suggestion[%d] = %q; want %q", i, s.Text, expected[i])
		}
		if s.Source != "llm" {
			t.Errorf("expected source 'llm', got %q", s.Source)
		}
	}
}
