package main

import (
	"fmt"
	"os"
	"memcleaner/internal/system"
	"memcleaner/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	mgr := system.NewPlatformManager()
	p := tea.NewProgram(tui.NewModel(mgr), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running MemCleaner: %v", err)
		os.Exit(1)
	}
}
