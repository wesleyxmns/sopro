package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme stays terminal-native and uses color only to communicate meaning.
type Theme struct {
	Brand      lipgloss.Style
	Focus      lipgloss.Style
	Selected   lipgloss.Style
	Good       lipgloss.Style
	Warning    lipgloss.Style
	Danger     lipgloss.Style
	Muted      lipgloss.Style
	Strong     lipgloss.Style
	Divider    lipgloss.Style
	FocusColor lipgloss.TerminalColor
	MutedColor lipgloss.TerminalColor
}

func NewTheme() Theme {
	mode := os.Getenv("SOPRO_THEME")
	theme, err := ThemeFor(mode)
	if err != nil {
		theme, _ = ThemeFor("auto")
	}
	return theme
}

func ThemeFor(mode string) (Theme, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "", "auto", "dark", "light", "mono", "cyber":
	default:
		return Theme{}, fmt.Errorf("tema %q inválido: use auto, dark, light, mono ou cyber", mode)
	}
	if os.Getenv("NO_COLOR") != "" || mode == "mono" {
		return monochromeTheme(), nil
	}

	switch mode {
	case "", "auto":
		return colorTheme(
			lipgloss.AdaptiveColor{Light: "#0369A1", Dark: "#67E8F9"},
			lipgloss.AdaptiveColor{Light: "#64748B", Dark: "#7F91A1"},
			lipgloss.AdaptiveColor{Light: "#166534", Dark: "#86EFAC"},
			lipgloss.AdaptiveColor{Light: "#92400E", Dark: "#FBBF24"},
			lipgloss.AdaptiveColor{Light: "#991B1B", Dark: "#FCA5A5"},
			lipgloss.AdaptiveColor{Light: "#E0F2FE", Dark: "#102A33"},
		), nil
	case "dark":
		return colorTheme(
			lipgloss.Color("#67E8F9"), lipgloss.Color("#7F91A1"),
			lipgloss.Color("#86EFAC"), lipgloss.Color("#FBBF24"),
			lipgloss.Color("#FCA5A5"), lipgloss.Color("#102A33"),
		), nil
	case "light":
		return colorTheme(
			lipgloss.Color("#0369A1"), lipgloss.Color("#64748B"),
			lipgloss.Color("#166534"), lipgloss.Color("#92400E"),
			lipgloss.Color("#991B1B"), lipgloss.Color("#E0F2FE"),
		), nil
	case "cyber":
		return colorTheme(
			lipgloss.Color("#7AA2F7"), lipgloss.Color("#565F89"),
			lipgloss.Color("#9ECE6A"), lipgloss.Color("#FF9E64"),
			lipgloss.Color("#F7768E"), lipgloss.Color("#1F2B46"),
		), nil
	}
	return Theme{}, nil
}

func colorTheme(focus, muted, good, warning, danger, selected lipgloss.TerminalColor) Theme {
	return Theme{
		Brand:      lipgloss.NewStyle().Foreground(focus).Bold(true),
		Focus:      lipgloss.NewStyle().Foreground(focus).Bold(true),
		Selected:   lipgloss.NewStyle().Background(selected),
		Good:       lipgloss.NewStyle().Foreground(good),
		Warning:    lipgloss.NewStyle().Foreground(warning),
		Danger:     lipgloss.NewStyle().Foreground(danger).Bold(true),
		Muted:      lipgloss.NewStyle().Foreground(muted),
		Strong:     lipgloss.NewStyle().Bold(true),
		Divider:    lipgloss.NewStyle().Foreground(muted),
		FocusColor: focus,
		MutedColor: muted,
	}
}

func monochromeTheme() Theme {
	return Theme{
		Brand:    lipgloss.NewStyle().Bold(true),
		Focus:    lipgloss.NewStyle().Bold(true),
		Selected: lipgloss.NewStyle().Bold(true),
		Good:     lipgloss.NewStyle(),
		Warning:  lipgloss.NewStyle().Bold(true),
		Danger:   lipgloss.NewStyle().Bold(true),
		Muted:    lipgloss.NewStyle().Faint(true),
		Strong:   lipgloss.NewStyle().Bold(true),
		Divider:  lipgloss.NewStyle().Faint(true),
	}
}
