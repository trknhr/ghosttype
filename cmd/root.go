package cmd

import (
	"database/sql"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/trknhr/ghosttype/internal"
	"github.com/trknhr/ghosttype/internal/history"
	"github.com/trknhr/ghosttype/internal/logger"
	"github.com/trknhr/ghosttype/internal/store"
	"github.com/trknhr/ghosttype/internal/tui"
	"github.com/trknhr/ghosttype/internal/worker"
)

func NewRootCmd(db *sql.DB, historyStore store.HistoryStore, historyLoader history.HistoryLoader) *cobra.Command {
	var filterModels string
	var quickExit bool

	go internal.SyncAliasesAsync(db)
	cmd := &cobra.Command{
		Use:   "ghosttype",
		Short: "Launch TUI for command suggestions",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			initial := ""
			if len(args) > 0 {
				initial = args[0]
			}

			model, err := tui.NewTuiModel(db, initial, filterModels, historyStore, historyLoader)
			if err != nil {
				return err
			}
			tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
			if err != nil {
				logger.Error("%v", err)
			}
			defer tty.Close()

			if quickExit {
				logger.Info("Quick exit mode: TUI initialization skipped\n")

				return nil
			}

			p := tea.NewProgram(model, tea.WithAltScreen(),
				tea.WithInput(tty),
				tea.WithOutput(os.Stderr),
			)
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("failed to run TUI: %w", err)
			}
			fmt.Println(model.SelectedText())
			return nil
		},
	}
	cmd.Flags().StringVar(&filterModels, "filter-models", "", "[dev] comma-separated model list to use (markov,freq,llm,alias,context)")
	cmd.Flags().BoolVar(&quickExit, "quick-exit", false, "Start and immediately exit (for benchmarking)")

	return cmd
}

func Execute(db *sql.DB) error {

	historyStore := store.NewSQLHistoryStore(db)
	hitoryLoader := history.NewHistoryLoaderAuto()
	worker.LaunchWorker(historyStore, hitoryLoader)

	cmd := NewRootCmd(db, historyStore, hitoryLoader)
	cmd.AddCommand(NewEvalCmd(db))

	cmd.AddCommand(generateCmd)
	cmd.AddCommand(NewBatchEvalCmd(db))
	cmd.AddCommand(NewEnsembleEvalCmd(db))
	cmd.AddCommand(NewBenchmarkCmd(db))
	cmd.AddCommand(NewProfileCmd(db))

	return cmd.Execute()
}
