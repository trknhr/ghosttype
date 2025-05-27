// cmd/tui.go
package cmd

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/trknhr/ghosttype/model"
)

type tuiFlags struct {
	outFile string
}

var flags tuiFlags

func init() {
	TuiCmd.Flags().StringVar(&flags.outFile, "out-file", "", "write selection to file instead of stdout (script‑friendly)")

}

type suggestionItem struct{ text string }

func (i suggestionItem) Title() string       { return i.text }
func (i suggestionItem) Description() string { return "" }
func (i suggestionItem) FilterValue() string { return i.text }

var TuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch TUI for command suggestions",
	RunE: func(cmd *cobra.Command, args []string) error {
		model, err := NewTuiModel(globalDB)
		if err != nil {
			return err
		}
		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("failed to run TUI: %w", err)
		}
		// fmt.Println(model.SelectedText())
		if flags.outFile != "" {
			f, err := os.Create(flags.outFile)
			if err != nil {
				return err
			}
			fmt.Fprintln(f, model.SelectedText())
			f.Close()
		} else {
			fmt.Println(model.SelectedText())
		}
		return nil
	},
}

// TUI model
type tuiModel struct {
	input     textinput.Model
	list      list.Model
	results   []model.Suggestion
	db        *sql.DB
	engine    model.SuggestModel
	lastInput string
	width     int
	height    int
	selected  string
}

type compactDelegate struct{}

func (d compactDelegate) Height() int                               { return 1 }
func (d compactDelegate) Spacing() int                              { return 0 }
func (d compactDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d compactDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(suggestionItem)
	if !ok {
		return
	}
	str := i.text
	if index == m.Index() {
		str = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render("> " + str)
	} else {
		str = "  " + str
	}
	fmt.Fprint(w, str)
}

func NewTuiModel(db *sql.DB) (*tuiModel, error) {
	input := textinput.New()
	input.Placeholder = "Type command prefix..."
	input.Focus()

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))

	// Expand to terminal width and height dynamically
	l := list.New([]list.Item{}, &compactDelegate{}, 40, 10)

	l.SetShowPagination(false)
	l.SetShowHelp(false)

	l.Title = "Suggestions"

	engine := GenerateModel(db, "") // 実際は適切なモデルを入れてください

	return &tuiModel{
		input:  input,
		list:   l,
		db:     db,
		engine: engine,
	}, nil
}

func (m *tuiModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			if !m.input.Focused() {
				if item, ok := m.list.SelectedItem().(suggestionItem); ok {
					m.selected = item.text
					return m, tea.Quit
				}
			}

		case "up", "down":
			m.input.Blur()
		default:
			if !m.input.Focused() {
				// 再度入力状態へ
				m.input.Focus()
			}

			if !strings.Contains(msg.String(), "[A") && !strings.Contains(msg.String(), "[B") {
				m.input, _ = m.input.Update(msg)
			}

		}
	}

	m.list, _ = m.list.Update(msg)

	prefix := strings.TrimSpace(m.input.Value())
	if prefix != m.lastInput {
		m.lastInput = prefix
		if prefix != "" {
			suggestions, err := m.engine.Predict(prefix)
			if err != nil {
				m.list.SetItems([]list.Item{suggestionItem{text: "Error: " + err.Error()}})
			} else {
				items := make([]list.Item, 0, len(suggestions))
				for _, s := range suggestions {
					items = append(items, suggestionItem{text: s.Text})
				}
				m.list.SetItems(items)
				m.list.ResetSelected()
			}
		} else {
			m.list.SetItems([]list.Item{})
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *tuiModel) View() string {
	s := "Ghosttype TUI\n\n"
	s += m.input.View() + "\n\n"
	s += m.list.View() + "\n"
	s += "(q = Quit)"
	return s
}
func (m *tuiModel) SelectedText() string {
	return m.selected
}
