package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trknhr/ghosttype/history"
	"github.com/trknhr/ghosttype/model"
	"github.com/trknhr/ghosttype/model/ensemble"
	"github.com/trknhr/ghosttype/model/freq"
	"github.com/trknhr/ghosttype/model/markov"
)

var rootCmd = &cobra.Command{
	Use:   "ghosttype <prefix>",
	Short: "Suggest command completions based on shell history",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		prefix := strings.TrimSpace(os.Args[1])

		// 履歴を読み込む
		historyPath := os.Getenv("HOME") + "/.zsh_history"
		historyEntries, err := history.LoadZshHistoryCommands(historyPath)
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

		// アンサンブル（仮に重みは固定値）
		wrappedMarkov := wrapModel("markov", 0.6, func(input string) []model.Suggestion {
			return markovModel.Predict(input)
		})
		wrappedFreq := wrapModel("freq", 0.4, func(input string) []model.Suggestion {
			return freqModel.Predict(input)
		})

		model := ensemble.New(wrappedMarkov, wrappedFreq)

		// 推論
		results := model.Predict(prefix)
		for _, r := range results {
			fmt.Println(r)
		}
		// prefix := strings.TrimSpace(args[0])

		// // 履歴読み込み
		// rawCommands, err := history.LoadZshHistoryCommands(os.Getenv("HOME") + "/.zsh_history")
		// if err != nil {
		// 	fmt.Fprintln(os.Stderr, "Failed to load history:", err)
		// 	os.Exit(1)
		// }

		// // Format command
		// var commands []string
		// for _, line := range rawCommands {
		// 	splits := strings.FieldsFunc(line, func(r rune) bool {
		// 		return r == ';' || r == '&' || r == '|'
		// 	})
		// 	for _, cmd := range splits {
		// 		cmd = strings.TrimSpace(cmd)
		// 		if cmd != "" {
		// 			commands = append(commands, cmd)
		// 		}
		// 	}
		// }

		// model := markov.NewModel()
		// model.Learn(commands)

		// // Generate suggestion with prefix
		// for _, suggestion := range model.PredictNext(prefix) {
		// 	fmt.Printf("%s %s\n", prefix, suggestion)
		// }
	},
}

func Execute() error {
	return rootCmd.Execute()
}

// wrapModel はマルコフやFreqのような[]stringを返すモデルをemsemble.SuggestModelに変換する
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
	// raw := m.predictFunc(input)
	// suggestions := make([]model.Suggestion, len(raw))
	// for i, text := range raw {
	// 	// 固定スコアで出す（アンサンブル側でweightがかかる）
	// 	suggestions[i] = model.Suggestion{
	// 		Text:   strings.TrimSpace(input + " " + text),
	// 		Source: m.source,
	// 		Score:  1.0,
	// 	}
	// }
	// return suggestions
}

func (m *wrappedModel) Weight() float64 {
	return m.weightValue
}
