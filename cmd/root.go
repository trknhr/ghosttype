package cmd

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"github.com/trknhr/ghosttype/history"
	"github.com/trknhr/ghosttype/internal"
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

var filterModels string

func init() {
	rootCmd.Flags().StringVar(&filterModels, "filter-models", "", "[dev] comma-separated model list to use (markov,freq,llm,alias,context)")
}

var globalDB *sql.DB

func isValidUTF8(s string) bool {
	return utf8.ValidString(s)
}

var rootCmd = &cobra.Command{
	Use:   "ghosttype <prefix>",
	Short: "Suggest command completions based on shell history",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		prefix := strings.TrimSpace(args[0])

		// Loding history
		historyPath := os.Getenv("HOME") + "/.zsh_history"
		historyEntries, err := history.LoadFilteredZshHistory(historyPath)
		if err != nil {
			log.Fatalf("failed to load history: %v", err)
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
			// Turn on all models
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
			models = append(models, alias.NewAliasModel(alias.NewSQLAliasStore(globalDB)))
		}
		if enabled["context"] {
			root, _ := os.Getwd()
			models = append(models, context.NewContextModelFromDir(root))
		}
		if enabled["llm"] {
			// latency of llm prediction is bad. TBD
			// models = append(models, llm.NewLLMRemoteModel("llama3.2", 2.0))
		}

		if enabled["embedding"] {
			m := embedding.NewModel(embedding.NewEmbeddingStore(globalDB), ollamaClient)
			m.Learn(cleaned)
			models = append(models, m)
		}

		model := ensemble.New(models...)

		// Predict
		results, err := model.Predict(prefix)
		if err != nil {
			logger.Error("some models failed during Predict: %v", err)
		}

		for _, r := range results {
			fmt.Println(r.Text)
		}
	},
}

func Execute(db *sql.DB) error {
	globalDB = db

	go internal.SyncAliasesAsync(db)
	rootCmd.AddCommand(TuiCmd)

	return rootCmd.Execute()
}
