package tui

import (
	"database/sql"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/trknhr/ghosttype/internal"
	"github.com/trknhr/ghosttype/model"
)

type tuiModel struct {
	input     textinput.Model
	list      list.Model
	db        *sql.DB
	engine    model.SuggestModel
	lastInput string
	width     int
	height    int
	selected  string
}

// compactDelegate renders items in a single-line compact form.
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

type suggestionItem struct{ text string }

func (i suggestionItem) Title() string       { return i.text }
func (i suggestionItem) Description() string { return "" }
func (i suggestionItem) FilterValue() string { return i.text }

func NewTuiModel(db *sql.DB, initialInput string, filterModels string) (*tuiModel, error) {
	input := textinput.New()
	input.Placeholder = "Type command prefix..."
	input.SetValue(initialInput)
	input.Focus()

	l := list.New([]list.Item{}, &compactDelegate{}, 40, 10)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)

	engine := internal.GenerateModel(db, filterModels)

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

type suggestionsMsg struct {
	prefix      string
	suggestions []model.Suggestion
	err         error
}

func fetchSuggestionsCmd(engine model.SuggestModel, prefix string) tea.Cmd {
	return func() tea.Msg {
		suggestions, err := engine.Predict(prefix)
		return suggestionsMsg{prefix, suggestions, err}
	}
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-4)

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEnter:
			if item, ok := m.list.SelectedItem().(suggestionItem); ok {
				m.selected = item.text
				m.input.SetValue(item.text)
				return m, tea.Quit
			}

		case tea.KeyUp, tea.KeyDown:
			m.input.Blur()

		default:
			if !m.input.Focused() {
				m.input.Focus()
			}
			m.input, _ = m.input.Update(msg)
		}
	case suggestionsMsg:
		if msg.prefix != m.lastInput {
			// discard outdated suggestions
			return m, nil
		}
		if msg.err != nil {
			m.list.SetItems([]list.Item{suggestionItem{text: "Error: " + msg.err.Error()}})
		} else {
			items := make([]list.Item, 0, len(msg.suggestions))
			for _, s := range msg.suggestions {
				items = append(items, suggestionItem{text: s.Text})
			}
			m.list.SetItems(items)
			m.list.ResetSelected()
		}
	}

	m.list, _ = m.list.Update(msg)

	prefix := strings.TrimSpace(m.input.Value())
	if prefix != m.lastInput {
		m.lastInput = prefix
		if prefix != "" {
			cmds = append(cmds, fetchSuggestionsCmd(m.engine, prefix))
		} else {
			m.list.SetItems([]list.Item{})
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *tuiModel) View() string {
	s := "Ghosttype\n\n"
	s += m.input.View() + "\n\n"
	s += m.list.View() + "\n"
	s += "(q = Ctrl+C)"
	return s
}

func (m *tuiModel) SelectedText() string {
	return m.selected
}
