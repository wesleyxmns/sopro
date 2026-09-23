package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const wordmark = "≋ SOPRO"

const logo = ` ████  ███  ████  ████   ███
█ ░░░░█ ░░█ █░░░█ █░░░█ █ ░░█
 ███░░█░ ░█░████░░████░░█░ ░█░
  ░░█ █░░ █░█░░░░ █░░█░ █░░ █░░
████░░ ███ ░█░░░░░█░░░█░ ███ ░░
 ░░░░ ░ ░░░ ░░░    ░░  ░  ░░░ ░
  ░░░░   ░░░  ░     ░   ░  ░░░`

// The art doubles as a shade map: █ marks solid pixels, ░ marks shade
// pixels. Shade is painted as a dimmed full block rather than the ░ glyph,
// which terminal fonts often substitute from another face with mismatched
// metrics, shearing the rows.
func (m Model) renderLogo() string {
	solid := m.theme.Brand
	shade := lipgloss.NewStyle().Faint(true)
	if m.theme.FocusColor != nil {
		shade = lipgloss.NewStyle().Foreground(m.theme.FocusColor).Faint(true)
	}
	rawLines := strings.Split(logo, "\n")
	lines := make([]string, len(rawLines))
	for index, raw := range rawLines {
		var rendered, segment strings.Builder
		kind := 0
		flush := func() {
			if segment.Len() == 0 {
				return
			}
			text := segment.String()
			segment.Reset()
			switch kind {
			case 1:
				rendered.WriteString(solid.Render(text))
			case 2:
				rendered.WriteString(shade.Render(text))
			default:
				rendered.WriteString(text)
			}
		}
		for _, r := range raw {
			next := 0
			glyph := string(r)
			switch r {
			case '█':
				next = 1
			case '░':
				next, glyph = 2, "█"
			}
			if next != kind {
				flush()
				kind = next
			}
			segment.WriteString(glyph)
		}
		flush()
		lines[index] = rendered.String()
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderSplash() string {
	logo := m.renderLogo()
	message := m.theme.Muted.Render("observando a memória com calma")
	content := lipgloss.JoinVertical(lipgloss.Center, logo, "", message)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, content)
}
