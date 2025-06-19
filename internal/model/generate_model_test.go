package model_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/trknhr/ghosttype/internal/history"
	"github.com/trknhr/ghosttype/internal/model"
	"github.com/trknhr/ghosttype/internal/model/entity"
	"github.com/trknhr/ghosttype/internal/ollama"
	_ "github.com/tursodatabase/go-libsql"
)

func TestGenerateModel_WithMocks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mocked history loader
	mockLoader := history.NewMockHistoryLoader(ctrl)
	mockLoader.
		EXPECT().
		LoadTail(100).
		Return([]string{"git status", "go build && go test"}, nil)

	// Mocked ollama client
	mockOllama := ollama.NewMockOllamaClient(ctrl)
	mockOllama.EXPECT().
		Embed(gomock.Any()).
		Return(&ollama.OllamaEmbedResponse{Embedding: []float32{0.1, 0.2}}, nil).
		AnyTimes()

	mockOllama.
		EXPECT().
		Generate(gomock.Any()).
		Return(&ollama.OllamaCompleteResponse{
			Response: "git status\ngit commit\ngit push",
		}, nil)

	db, err := sql.Open("libsql", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS history (id INTEGER PRIMARY KEY, command TEXT, count INTEGER);`)
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS aliases (name TEXT, cmd TEXT, updated_at TIMESTAMP);`)

	ensembleModel, ch, err := model.GenerateModel(nil, mockLoader, mockOllama, db, "prefix,alias,llm,embedding")
	if err != nil {
		t.Fatalf("GenerateModel failed: %v", err)
	}

	models := ensembleModel.Models.Load()
	suggestModels, ok := models.([]entity.SuggestModel)

	if !ok {
		t.Fatalf("unexpected model type: %T", models)
	}

	if len(suggestModels) < 2 {
		t.Errorf("expected at least 2 models, got %d", len(suggestModels))
	}

	var llmReady, embeddingReady bool
	for i := 0; i < 2; i++ {
		select {
		case ev := <-ch:
			switch ev.Name {
			case "llm":
				if ev.Status != model.ModelReady {
					t.Errorf("llm model not ready: %+v", ev)
				}
				llmReady = true
			case "embedding":
				if ev.Status != model.ModelReady {
					t.Errorf("embedding model not ready: %+v", ev)
				}
				embeddingReady = true
			default:
				t.Errorf("unexpected model event: %v", ev.Name)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("timeout waiting for model init event")
		}
	}

	if !llmReady || !embeddingReady {
		t.Errorf("not all heavy models initialized: llm=%v, embedding=%v", llmReady, embeddingReady)
	}
}
