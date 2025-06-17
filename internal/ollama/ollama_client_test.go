package ollama_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trknhr/ghosttype/internal/ollama"
)

func TestHTTPClient_Embed(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embeddings" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body["model"] != "test-embed-model" || !strings.Contains(body["prompt"], "test input") {
			t.Errorf("unexpected body: %+v", body)
		}

		resp := map[string][]float32{
			"embedding": {0.1, 0.2, 0.3},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	client := &ollama.HTTPClient{
		Model:          "unused",
		EmbeddingModel: "test-embed-model",
		BaseURL:        mockServer.URL,
		Client:         mockServer.Client(),
	}

	resp, err := client.Embed("test input")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(resp.Embedding) != 3 || resp.Embedding[0] != 0.1 {
		t.Errorf("unexpected embedding: %+v", resp.Embedding)
	}
}

func TestHTTPClient_Generate(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if body["model"] != "test-llm-model" || !strings.Contains(fmt.Sprint(body["prompt"]), "test prompt") {
			t.Errorf("unexpected body: %+v", body)
		}

		resp := map[string]string{
			"response": "echo hello\nls -al\nmake build",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	client := &ollama.HTTPClient{
		Model:          "test-llm-model",
		EmbeddingModel: "unused",
		BaseURL:        mockServer.URL,
		Client:         mockServer.Client(),
	}

	resp, err := client.Generate("test prompt")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !strings.Contains(resp.Response, "echo") {
		t.Errorf("unexpected response: %s", resp.Response)
	}
}
