package ui

import (
	"github.com/trknhr/markov-cli"

	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	input       string
	suggestions []string
	cursor      int
	markovModel *markov.Model
}

func InitialModel() Model {
	model := markov.NewModel()
	sampleHistory := []string{
		"cd project",
		"ls",
		"git status",
		"git add .",
		"git commit",
		"cd project",
		"ls",
		"docker ps",
		"docker exec -it db bash",
		"exit",
	}
	model.Learn(sampleHistory)

	return Model{
		input:       "",
		suggestions: sampleHistory[:3],
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
			m.input = m.suggestions[m.cursor]
		default:
			if len(msg.String()) == 1 {
				m.input += msg.String()
			} else if msg.String() == "backspace" && len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}

			if m.markovModel != nil {
				pred := m.markovModel.PredictNext(m.input)
				if pred != "" {
					m.suggestions = []string{pred}
					m.cursor = 0
				}
			}
		}
	}
	return m, nil
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
