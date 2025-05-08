package main

import (
	"log"

	"github.com/trknhr/markov-cli/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	p := tea.NewProgram(ui.InitialModel())
	if err := p.Start(); err != nil {
		log.Fatal(err)
	}
}
