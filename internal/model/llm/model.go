package llm

import (
	"fmt"
	"strings"

	"github.com/trknhr/ghosttype/internal/model/entity"
	"github.com/trknhr/ghosttype/internal/ollama"
)

type LLMRemoteModel struct {
	client ollama.OllamaClient
}

func NewLLMRemoteModel(client ollama.OllamaClient) entity.SuggestModel {
	return &LLMRemoteModel{
		client: client,
	}
}

func (m *LLMRemoteModel) Predict(input string) ([]entity.Suggestion, error) {
	prompt := fmt.Sprintf(`You are a shell autocomplete engine.

Your task is to suggest exactly 3 possible shell commands that begin with the given prefix.

- Output exactly 5 candidates
- One per line
- No markdown, quotes, or extra formatting
- No explanations or context
- If the prefix is empty, return 3 common shell commands

Prefix: %q
`, input)

	resp, err := m.client.Generate(prompt)

	if err != nil {
		return nil, err
	}

	lines := strings.Split(resp.Response, "\n")

	var suggestions []entity.Suggestion
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			suggestions = append(suggestions, entity.Suggestion{
				Text:   line,
				Source: "llm",
				Score:  1.0,
			})
		}
	}
	return suggestions, nil
}

func (m *LLMRemoteModel) Learn(entries []string) error {
	return nil
}

func (m *LLMRemoteModel) Weight() float64 {
	return 0.5
}
