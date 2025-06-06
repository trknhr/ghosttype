package internal

import (
	"database/sql"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unicode/utf8"

	"github.com/trknhr/ghosttype/history"
	"github.com/trknhr/ghosttype/internal/logger.go"
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
	historyEntries, err := history.LoadZshHistoryTail(historyPath, 100)
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

	launchWorker()

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
		m := freq.NewFreqModel(db)
		err := m.Learn(cleaned)
		if err != nil {
			logger.Error("failed to learn frequency model: %v", err)
		}
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

func SaveHistory(db *sql.DB, entries []string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

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

var workerOnce sync.Once

func launchWorker() {
	workerOnce.Do(func() {
		go func() {
			logger.Debug("launching learn-history worker")
			cmd := exec.Command(os.Args[0], "load-history")

			// 共通設定
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = nil

			if runtime.GOOS == "windows" {
				// Windows: detached mode を有効に
				// TBD
				// cmd.SysProcAttr = &syscall.SysProcAttr{
				// 	CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP |
				// 		syscall.CREATE_NO_WINDOW,
				// }
			} else {
				// macOS/Linux: セッションとプロセスグループを切り離す
				cmd.SysProcAttr = &syscall.SysProcAttr{
					Setsid:  true,
					Setpgid: true,
				}
			}

			if err := cmd.Start(); err != nil {
				logger.Error("failed to launch learn-history: %v", err)
			}

		}()
	})

}
