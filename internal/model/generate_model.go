package model

import (
	"database/sql"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/trknhr/ghosttype/internal/history"
	"github.com/trknhr/ghosttype/internal/model/alias"
	"github.com/trknhr/ghosttype/internal/model/entity"
	"github.com/trknhr/ghosttype/internal/model/setting"
	"github.com/trknhr/ghosttype/internal/store"

	"github.com/trknhr/ghosttype/internal/model/embedding"
	"github.com/trknhr/ghosttype/internal/model/ensemble"
	"github.com/trknhr/ghosttype/internal/model/freq"
	"github.com/trknhr/ghosttype/internal/model/llm"
	"github.com/trknhr/ghosttype/internal/model/markov"
	"github.com/trknhr/ghosttype/internal/model/prefix"
	"github.com/trknhr/ghosttype/internal/ollama"
)

func GenerateModel(
	historyStore store.HistoryStore,
	historyLoader history.HistoryLoader,
	ollamaClient ollama.OllamaClient,
	db *sql.DB,
	filterModels string) (*ensemble.Ensemble, <-chan ModelInitEvent, error) {

	historyEntries, err := historyLoader.LoadTail(100)

	if err != nil {
		return nil, nil, err
	}

	var cleaned []string
	for _, entry := range historyEntries {
		splits := strings.FieldsFunc(entry, func(r rune) bool {
			return r == ';' || r == '|'
		})
		for _, s := range splits {
			s = strings.TrimSpace(s)
			if s != "" && utf8.ValidString(s) {
				cleaned = append(cleaned, s)
			}
		}
	}

	enabled := map[string]bool{}

	if filterModels == "" {
		enabled["markov"] = true
		enabled["freq"] = true
		enabled["prefix"] = true
		enabled["alias"] = true
		enabled["context"] = true
		enabled["llm"] = true
		enabled["embedding"] = true
	} else {
		for _, name := range strings.Split(filterModels, ",") {
			enabled[strings.TrimSpace(name)] = true
		}
	}

	var lightModels []entity.SuggestModel

	if enabled["markov"] {
		m := markov.NewMarkovModel()
		m.Learn(cleaned)
		lightModels = append(lightModels, m)
	}
	if enabled["freq"] {
		m := freq.NewFreqModel(db)
		lightModels = append(lightModels, m)
	}
	if enabled["prefix"] {
		m := prefix.NewPrefixModel(db)
		lightModels = append(lightModels, m)

	}
	if enabled["alias"] {
		lightModels = append(lightModels, alias.NewAliasModel(alias.NewSQLAliasStore(db)))
	}
	if enabled["context"] {
		root, _ := os.Getwd()
		lightModels = append(lightModels, setting.NewContextModelFromDir(root))
	}

	// Create the ensemble model with light models
	ensembleModel := ensemble.NewEnsemble(lightModels)

	ch := make(chan ModelInitEvent, 2) // bufferはモデル数分

	if enabled["embedding"] {
		m := embedding.NewModel(embedding.NewEmbeddingStore(db), ollamaClient)
		go func() {
			// test if the model is working
			_, err := ollamaClient.Embed("echo")
			if err != nil {
				ch <- ModelInitEvent{Name: "embedding", Status: ModelError, Err: err}
			} else {
				go m.Learn(cleaned)
				ensembleModel.AddHeavyModel(m)
				ch <- ModelInitEvent{Name: "embedding", Status: ModelReady, Err: nil}
			}
		}()
	}
	if enabled["llm"] {
		llmModel := llm.NewLLMRemoteModel(ollamaClient)

		go func() {
			_, err := llmModel.Predict("echo")
			if err != nil {
				// utils.WarnOnce()
				ch <- ModelInitEvent{Name: "llm", Status: ModelError, Err: err}
			} else {
				ensembleModel.AddHeavyModel(llmModel)
				ch <- ModelInitEvent{Name: "llm", Status: ModelReady, Err: nil}
			}
		}()
	}

	return ensembleModel, ch, nil
}
