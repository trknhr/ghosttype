package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trknhr/ghosttype/history"
	markov "github.com/trknhr/ghosttype/marcov"
)

var rootCmd = &cobra.Command{
	Use:   "zsh-predict <prefix>",
	Short: "Suggest command completions based on shell history",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		prefix := strings.TrimSpace(args[0])

		// 履歴読み込み
		rawCommands, err := history.LoadZshHistory(os.Getenv("HOME") + "/.zsh_history")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to load history:", err)
			os.Exit(1)
		}

		// コマンド整形
		var commands []string
		for _, line := range rawCommands {
			splits := strings.FieldsFunc(line, func(r rune) bool {
				return r == ';' || r == '&' || r == '|'
			})
			for _, cmd := range splits {
				cmd = strings.TrimSpace(cmd)
				if cmd != "" {
					commands = append(commands, cmd)
				}
			}
		}

		model := markov.NewModel()
		model.Learn(commands)

		// Generate suggestion with prefix
		for _, suggestion := range model.PredictNext(prefix) {
			fmt.Printf("%s %s\n", prefix, suggestion)
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}
