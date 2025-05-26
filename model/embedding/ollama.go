package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"
)

type ollamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// embedViaOllama sends the text to Ollama's /api/embeddings endpoint and returns the embedding vector.
func embedViaOllama(text string) []float32 {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reqBody, err := json.Marshal(ollamaEmbedRequest{
		Model:  "nomic-embed-text",
		Prompt: text,
	})
	if err != nil {
		log.Printf("[embed] marshal error: %v", err)
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "http://localhost:11434/api/embeddings", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("[embed] request creation failed: %v", err)
		return nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[embed] request failed: %v", err)
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[embed] read body failed: %v", err)
		return nil
	}

	var parsed ollamaEmbedResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		log.Printf("[embed] unmarshal error: %v\nbody=%s", err, string(body))
		return nil
	}

	return parsed.Embedding
}
