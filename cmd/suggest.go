// cmd/suggest.go を新規作成

package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/trknhr/ghosttype/history"
	"github.com/trknhr/ghosttype/internal/logger.go"
	"github.com/trknhr/ghosttype/model"
	"github.com/trknhr/ghosttype/model/alias"
	"github.com/trknhr/ghosttype/model/context"
	"github.com/trknhr/ghosttype/model/embedding"
	"github.com/trknhr/ghosttype/model/ensemble"
	"github.com/trknhr/ghosttype/model/freq"
	"github.com/trknhr/ghosttype/model/markov"
	"github.com/trknhr/ghosttype/ollama"
)

func GenerateModel(db *sql.DB, filterModels string) model.SuggestModel {
	historyPath := os.Getenv("HOME") + "/.zsh_history"
	historyEntries, err := history.LoadFilteredZshHistory(historyPath)
	if err != nil {
		// return nil, fmt.Errorf("failed to load history: %w", err)
		return nil
	}

	var cleaned []string
	for _, entry := range historyEntries {
		splits := strings.FieldsFunc(entry, func(r rune) bool {
			return r == ';' || r == '&' || r == '|'
		})
		for _, s := range splits {
			s = strings.TrimSpace(s)
			if s != "" && utf8.ValidString(s) {
				cleaned = append(cleaned, s)
			}
		}
	}

	ollamaClient := ollama.NewHTTPClient("llama3.2", "nomic-embed-text")
	enabled := map[string]bool{}

	if filterModels == "" {
		enabled["markov"] = true
		enabled["freq"] = true
		enabled["alias"] = true
		enabled["context"] = true
		enabled["llm"] = true
		enabled["embedding"] = true
	} else {
		for _, name := range strings.Split(filterModels, ",") {
			enabled[strings.TrimSpace(name)] = true
		}
	}

	var models []model.SuggestModel

	if enabled["markov"] {
		m := markov.NewModel()
		m.Learn(cleaned)
		models = append(models, m)
	}
	if enabled["freq"] {
		m := freq.NewModel()
		m.Learn(cleaned)
		models = append(models, m)
	}
	if enabled["alias"] {
		models = append(models, alias.NewAliasModel(alias.NewSQLAliasStore(db)))
	}
	if enabled["context"] {
		root, _ := os.Getwd()
		models = append(models, context.NewContextModelFromDir(root))
	}
	if enabled["embedding"] {
		m := embedding.NewModel(embedding.NewEmbeddingStore(db), ollamaClient)
		m.Learn(cleaned)
		models = append(models, m)
	}

	return ensemble.New(models...)
}
func SuggestFromPrefix(prefix string, db *sql.DB, filterModels string) ([]model.Suggestion, error) {
	historyPath := os.Getenv("HOME") + "/.zsh_history"
	historyEntries, err := history.LoadFilteredZshHistory(historyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load history: %w", err)
	}

	var cleaned []string
	for _, entry := range historyEntries {
		splits := strings.FieldsFunc(entry, func(r rune) bool {
			return r == ';' || r == '&' || r == '|'
		})
		for _, s := range splits {
			s = strings.TrimSpace(s)
			if s != "" && utf8.ValidString(s) {
				cleaned = append(cleaned, s)
			}
		}
	}

	ollamaClient := ollama.NewHTTPClient("llama3.2", "nomic-embed-text")
	enabled := map[string]bool{}

	if filterModels == "" {
		enabled["markov"] = true
		enabled["freq"] = true
		enabled["alias"] = true
		enabled["context"] = true
		enabled["llm"] = true
		enabled["embedding"] = true
	} else {
		for _, name := range strings.Split(filterModels, ",") {
			enabled[strings.TrimSpace(name)] = true
		}
	}

	var models []model.SuggestModel

	if enabled["markov"] {
		m := markov.NewModel()
		m.Learn(cleaned)
		models = append(models, m)
	}
	if enabled["freq"] {
		m := freq.NewModel()
		m.Learn(cleaned)
		models = append(models, m)
	}
	if enabled["alias"] {
		models = append(models, alias.NewAliasModel(alias.NewSQLAliasStore(db)))
	}
	if enabled["context"] {
		root, _ := os.Getwd()
		models = append(models, context.NewContextModelFromDir(root))
	}
	if enabled["embedding"] {
		m := embedding.NewModel(embedding.NewEmbeddingStore(db), ollamaClient)
		m.Learn(cleaned)
		models = append(models, m)
	}

	engine := ensemble.New(models...)

	results, err := engine.Predict(prefix)
	if err != nil {
		logger.Error("predict failed: %v", err)
	}
	return results, nil
}
