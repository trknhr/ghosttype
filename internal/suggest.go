package internal

import (
	"database/sql"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/trknhr/ghosttype/history"
	"github.com/trknhr/ghosttype/internal/utils"
	"github.com/trknhr/ghosttype/model"
	"github.com/trknhr/ghosttype/model/alias"
	"github.com/trknhr/ghosttype/model/context"
	"github.com/trknhr/ghosttype/model/embedding"
	"github.com/trknhr/ghosttype/model/ensemble"
	"github.com/trknhr/ghosttype/model/freq"
	"github.com/trknhr/ghosttype/model/llm"
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
		m := markov.NewMarkovModel()
		m.Learn(cleaned)
		models = append(models, m)
	}
	if enabled["freq"] {
		m := freq.NewFreqModel()
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
		// test if the model is working
		_, err := ollamaClient.Embed("echo")
		if err != nil {
			utils.WarnOnce()
		} else {
			go m.Learn(cleaned)
			models = append(models, m)
		}

	}
	if enabled["llm"] {
		llmModel := llm.NewLLMRemoteModel(ollamaClient)

		// test if the model is working
		_, err := llmModel.Predict("echo")
		if err != nil {
			utils.WarnOnce()
		} else {
			models = append(models, llmModel)
		}

	}

	return ensemble.New(models...)
}
