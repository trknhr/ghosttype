package ui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/trknhr/markov-cli/history"
	markov "github.com/trknhr/markov-cli/marcov"
)

type Model struct {
	input       string
	suggestions []string
	cursor      int
	markovModel *markov.Model
}

func InitialModel() Model {
	model := markov.NewModel()

	commands, err := history.LoadZshHistory(os.Getenv("HOME") + "/.zsh_history")
	fmt.Println(commands)
	if err != nil {
		fmt.Println("Failed to load history:", err)
		commands = []string{} // fallback
	}

	model.Learn(commands)

	return Model{
		input:       "",
		suggestions: commands[:min(3, len(commands))],
		cursor:      0,
		markovModel: model,
	}

}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.suggestions)-1 {
				m.cursor++
			}
		case "enter":
			selected := m.suggestions[m.cursor]
			if m.input != "" {
				m.input += " " + selected
			} else {
				m.input = selected
			}

			return m, tea.Quit
		default:
			if len(msg.String()) == 1 {
				m.input += msg.String()
			} else if msg.String() == "backspace" && len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}

			if m.markovModel != nil {
				preds := m.markovModel.PredictNext(m.input)
				if len(preds) > 0 {
					m.suggestions = preds
				} else {
					m.suggestions = []string{"(no match)"}
				}
			}
		}
	}
	return m, nil
}

func (m Model) Input() string {
	return m.input
}

func (m Model) View() string {
	s := "Command: " + m.input + "\n\nSuggestions:\n"
	for i, cmd := range m.suggestions {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}
		s += cursor + cmd + "\n"
	}
	s += "\n(↑/↓ to navigate, Enter to select, q to quit)"
	return s
}
