package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/trknhr/ghosttype/internal/logger.go"
	"github.com/trknhr/ghosttype/model"
)

type LLMRemoteModel struct {
	modelName string
	weight    float64
	client    *http.Client
}

func NewLLMRemoteModel(modelName string, weight float64) model.SuggestModel {
	return &LLMRemoteModel{
		modelName: modelName,
		weight:    weight,
		client:    &http.Client{Timeout: 5 * time.Second},
	}
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
}

func (m *LLMRemoteModel) Predict(input string) []model.Suggestion {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	logger.Debug("[llm] llm predict")

	prompt := fmt.Sprintf(`You are a command-line autocomplete engine.
Given a partial shell command, return exactly 3 likely completions. 
Respond with each candidate on its own line. Do not add explanations or examples.
Respond with only raw shell commands. No markdown, no numbers, no quotes.
Only output valid commands. Do not invent new ones.

Prefix: %q

Output:
`, input)

	reqBody, err := json.Marshal(ollamaRequest{
		Model:  m.modelName,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		logger.Debug("[llm] request marshal failed: %v", err)
		return nil
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "http://localhost:11434/api/generate", bytes.NewBuffer(reqBody))
	if err != nil {
		logger.Debug("[llm] http request creation failed: %v", err)
		return nil
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(httpReq)
	if err != nil {
		logger.Debug("[llm] ollama request failed: %v", err)
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Debug("[llm] read error: %v", err)
		return nil
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		logger.Debug("[llm] parse error: %v", err)
		return nil
	}

	logger.Debug("[llm] response: %v", ollamaResp.Response)
	lines := strings.Split(ollamaResp.Response, "\n")

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
	return suggestions
}

func (m *LLMRemoteModel) Learn(entries []string) {
	// read-only
}

func (m *LLMRemoteModel) Weight() float64 {
	return m.weight
}
