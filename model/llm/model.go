package llm

import (
	"fmt"
	"strings"

	"github.com/trknhr/ghosttype/model"
	"github.com/trknhr/ghosttype/ollama"
)

type LLMRemoteModel struct {
	client ollama.OllamaClient
}

func NewLLMRemoteModel(client ollama.OllamaClient) model.SuggestModel {
	return &LLMRemoteModel{
		client: client,
	}
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

func (m *LLMRemoteModel) Predict(input string) ([]model.Suggestion, error) {
	prompt := fmt.Sprintf(`You are a shell autocomplete engine.

Your task is to suggest exactly 3 possible shell commands that begin with the given prefix.

- Output exactly 3 candidates
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

	var suggestions []model.Suggestion
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			suggestions = append(suggestions, model.Suggestion{
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
