package tui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	Background lipgloss.Style
	Header     lipgloss.Style
	Title      lipgloss.Style
	Active     lipgloss.Style
	Safe       lipgloss.Style
	Warning    lipgloss.Style
	Danger     lipgloss.Style
	Muted      lipgloss.Style
	Border     lipgloss.Border
}

var SoproDS = Theme{
	Background: lipgloss.NewStyle().Background(lipgloss.Color("#1A1B26")),
	Header:     lipgloss.NewStyle().Foreground(lipgloss.Color("#7AA2F7")).Bold(true),
	Title:      lipgloss.NewStyle().Foreground(lipgloss.Color("#7AA2F7")).Bold(true).Padding(0, 1),
	Active:     lipgloss.NewStyle().Foreground(lipgloss.Color("#1A1B26")).Background(lipgloss.Color("#7AA2F7")).Bold(true),
	Safe:       lipgloss.NewStyle().Foreground(lipgloss.Color("#9ECE6A")),
	Warning:    lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9E64")),
	Danger:     lipgloss.NewStyle().Foreground(lipgloss.Color("#F7768E")),
	Muted:      lipgloss.NewStyle().Foreground(lipgloss.Color("#565F89")),
	Border:     lipgloss.RoundedBorder(),
}
