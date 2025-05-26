package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/trknhr/ghosttype/internal/logger.go"
)

type OllamaCompleteResponse struct {
	Response string `json:"response"`
}

type OllamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

type OllamaClient interface {
	Embed(text string) (*OllamaEmbedResponse, error)
	Generate(prompt string) (*OllamaCompleteResponse, error)
}

type HTTPClient struct {
	Model          string
	EmbeddingModel string
	BaseURL        string
	Client         *http.Client
}

func NewHTTPClient(model string, embeddingModel string) *HTTPClient {
	return &HTTPClient{
		Model:          model,
		EmbeddingModel: embeddingModel,
		BaseURL:        "http://localhost:11434",
		Client:         &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *HTTPClient) Embed(text string) (*OllamaEmbedResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reqBody, err := json.Marshal(map[string]string{
		"model":  c.EmbeddingModel,
		"prompt": text,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/api/embeddings", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read embed response: %w", err)
	}

	var parsed struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse embed response: %w\nbody=%s", err, string(body))
	}

	return &OllamaEmbedResponse{
		Embedding: parsed.Embedding,
	}, nil
}

func (c *HTTPClient) Generate(prompt string) (*OllamaCompleteResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	reqBody, err := json.Marshal(map[string]interface{}{
		"model":  c.Model,
		"prompt": prompt,
		"stream": false,
	})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/api/generate", bytes.NewBuffer(reqBody))
	if err != nil {
		logger.Debug("[llm] http request creation failed: %v", err)
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(httpReq)
	if err != nil {
		logger.Debug("[llm] ollama request failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Debug("[llm] read error: %v", err)
		return nil, err
	}

	var ollamaResp OllamaCompleteResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		logger.Debug("[llm] parse error: %v", err)
		return nil, err
	}

	return &ollamaResp, nil
}
