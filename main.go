package main

import (
	"fmt"
	"os"

	"github.com/trknhr/markov-cli/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// p := tea.NewProgram(ui.InitialModel())
	// if err := p.Start(); err != nil {
	// 	log.Fatal(err)
	// }
	p := tea.NewProgram(ui.InitialModel(), tea.WithAltScreen()) // optional: alt screen

	finalModel, err := p.Run()
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	model := finalModel.(ui.Model)

	// 👇 画面をクリアするANSIコード
	fmt.Print("\033[H\033[2J")

	// 👇 選ばれたコマンドだけを表示
	fmt.Println(model.Input())
}
