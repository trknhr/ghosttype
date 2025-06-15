package internal

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/trknhr/ghosttype/history"
	"github.com/trknhr/ghosttype/internal/logger.go"
	"github.com/trknhr/ghosttype/internal/utils"
	"github.com/trknhr/ghosttype/model"
	"github.com/trknhr/ghosttype/model/alias"
	"github.com/trknhr/ghosttype/model/setting"

	"github.com/trknhr/ghosttype/model/embedding"
	"github.com/trknhr/ghosttype/model/ensemble"
	"github.com/trknhr/ghosttype/model/freq"
	"github.com/trknhr/ghosttype/model/llm"
	"github.com/trknhr/ghosttype/model/markov"
	"github.com/trknhr/ghosttype/model/prefix"
	"github.com/trknhr/ghosttype/ollama"
)

func GenerateModel(db *sql.DB, filterModels string) *ensemble.Ensemble {
	historyPath := os.Getenv("HOME") + "/.zsh_history"
	historyEntries, err := history.LoadZshHistoryTail(historyPath, 100)
	if err != nil {
		// return nil, fmt.Errorf("failed to load history: %w", err)
		return nil
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

	// launchWorker(db)

	ollamaClient := ollama.NewHTTPClient("llama3.2:1b", "nomic-embed-text")
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

	var lightModels []model.SuggestModel
	var heavyModels []model.SuggestModel

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
	if enabled["embedding"] {
		m := embedding.NewModel(embedding.NewEmbeddingStore(db), ollamaClient)
		// test if the model is working
		_, err := ollamaClient.Embed("echo")
		if err != nil {
			utils.WarnOnce()
		} else {
			go m.Learn(cleaned)
			heavyModels = append(heavyModels, m)
		}

	}
	if enabled["llm"] {
		llmModel := llm.NewLLMRemoteModel(ollamaClient)

		// test if the model is working
		_, err := llmModel.Predict("echo")
		if err != nil {
			utils.WarnOnce()
		} else {
			heavyModels = append(heavyModels, llmModel)
		}

	}

	return ensemble.NewWithClassification(lightModels, heavyModels)
}

func SaveHistory(db *sql.DB, entries []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO history(command, hash, count)
		VALUES (?, ?, 1)
		ON CONFLICT(hash) DO UPDATE SET count = count + 1
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, cmd := range entries {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		hash := utils.Hash(cmd)
		if _, err := stmt.Exec(cmd, hash); err != nil {
			logger.Error("failed to insert command: %s, %v", cmd, err)
		}
	}
	if err := tx.Commit(); err != nil {
		logger.Error("failed to commit history tx: %v", err)
		return err
	}

	return nil
}

func launchWorker(db *sql.DB) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		if err := RunHistoryWorker(ctx, db); err != nil {
			logger.Error("background learning failed: %v", err)
		}
	}()
}

func RunHistoryWorker(ctx context.Context, db *sql.DB) error {
	historyPath := os.Getenv("HOME") + "/.zsh_history"
	historyEntries, err := history.LoadZshHistoryCommands(historyPath)
	logger.Debug("loaded %d history entries from %s", len(historyEntries), historyPath)
	if err != nil {
		return err
	}

	var cleaned []string
	for _, entry := range historyEntries {
		s := strings.TrimSpace(entry)
		if s != "" && utf8.ValidString(s) {
			cleaned = append(cleaned, s)
		}
	}

	return SaveHistory(db, cleaned)
}
