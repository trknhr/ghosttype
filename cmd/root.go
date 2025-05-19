package cmd

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trknhr/ghosttype/history"
	"github.com/trknhr/ghosttype/internal"
	"github.com/trknhr/ghosttype/model"
	"github.com/trknhr/ghosttype/model/alias"
	"github.com/trknhr/ghosttype/model/context"
	"github.com/trknhr/ghosttype/model/ensemble"
	"github.com/trknhr/ghosttype/model/freq"
	"github.com/trknhr/ghosttype/model/markov"
)

var globalDB *sql.DB

var rootCmd = &cobra.Command{
	Use:   "ghosttype <prefix>",
	Short: "Suggest command completions based on shell history",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		prefix := strings.TrimSpace(os.Args[1])

		// 履歴を読み込む
		historyPath := os.Getenv("HOME") + "/.zsh_history"
		historyEntries, err := history.LoadFilteredZshHistory(historyPath)
		if err != nil {
			log.Fatalf("failed to load history: %v", err)
		}

		// コマンド分割（セミコロンや &, | で分割）
		var cleaned []string
		for _, entry := range historyEntries {
			splits := strings.FieldsFunc(entry, func(r rune) bool {
				return r == ';' || r == '&' || r == '|'
			})
			for _, s := range splits {
				s = strings.TrimSpace(s)
				if s != "" {
					cleaned = append(cleaned, s)
				}
			}
		}

		// モデル初期化
		markovModel := markov.NewModel()
		markovModel.Learn(cleaned)

		freqModel := freq.NewModel()
		freqModel.Learn(cleaned)

		aliasModel := alias.NewAliasModel(globalDB)

		root, err := os.Getwd()
		if err != nil {
			log.Fatalf("failed to get working directory: %v", err)
		}
		contextModel := context.NewContextModelFromDir(root)

		model := ensemble.New(
			markovModel,
			freqModel,
			aliasModel,
			contextModel,
		)

		// Predict
		results := model.Predict(prefix)
		for _, r := range results {
			fmt.Println(r)
		}
	},
}

func Execute(db *sql.DB) error {
	globalDB = db

	go internal.SyncAliasesAsync(db)

	return rootCmd.Execute()
}

func wrapModel(source string, weight float64, predictFunc func(string) []model.Suggestion) model.SuggestModel {
	return &wrappedModel{
		source:      source,
		weightValue: weight,
		predictFunc: predictFunc,
	}
}

type wrappedModel struct {
	source      string
	weightValue float64
	predictFunc func(string) []model.Suggestion
}

func (m *wrappedModel) Learn([]string) {}

func (m *wrappedModel) Predict(input string) []model.Suggestion {
	return m.predictFunc(input)
}

func (m *wrappedModel) Weight() float64 {
	return m.weightValue
}
