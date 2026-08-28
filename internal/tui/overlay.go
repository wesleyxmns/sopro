package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// overlayCentered composes a foreground block over an already-rendered view.
// ANSI-aware cuts preserve terminal cell widths and the styles on both sides.
func overlayCentered(background, foreground string, width, height int) string {
	backgroundLines := strings.Split(background, "\n")
	for len(backgroundLines) < height {
		backgroundLines = append(backgroundLines, "")
	}
	if len(backgroundLines) > height {
		backgroundLines = backgroundLines[:height]
	}

	foregroundLines := strings.Split(foreground, "\n")
	foregroundWidth := lipgloss.Width(foreground)
	foregroundHeight := len(foregroundLines)
	leftOffset := max((width-foregroundWidth)/2, 0)
	topOffset := max((height-foregroundHeight)/2, 0)

	for index, foregroundLine := range foregroundLines {
		row := topOffset + index
		if row >= len(backgroundLines) {
			break
		}

		backgroundLine := backgroundLines[row]
		left := ansi.Cut(backgroundLine, 0, leftOffset)
		left += strings.Repeat(" ", max(leftOffset-lipgloss.Width(left), 0))

		foregroundLine += strings.Repeat(" ", max(foregroundWidth-lipgloss.Width(foregroundLine), 0))
		rightStart := min(leftOffset+foregroundWidth, width)
		right := ansi.Cut(backgroundLine, rightStart, width)

		composed := left + foregroundLine + right
		composed += strings.Repeat(" ", max(width-lipgloss.Width(composed), 0))
		backgroundLines[row] = ansi.Truncate(composed, width, "")
	}

	return strings.Join(backgroundLines, "\n")
}
