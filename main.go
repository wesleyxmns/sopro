package main

import (
	"fmt"
	"os"
	"memcleaner/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	p := tea.NewProgram(tui.NewModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running MemCleaner: %v", err)
		os.Exit(1)
	}
}
