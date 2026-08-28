package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const wordmark = "≋ SOPRO"

const logo = `  ____   ___  ____  ____   ___
 / ___| / _ \|  _ \|  _ \ / _ \
 \___ \| | | | |_) | |_) | | | |
  ___) | |_| |  __/|  _ <| |_| |
 |____/ \___/|_|   |_| \_\ \___/`

func (m Model) renderLogo() string {
	logoLines := strings.Split(logo, "\n")
	lines := make([]string, len(logoLines))
	for index, line := range logoLines {
		lines[index] = m.theme.Brand.Render(line)
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderSplash() string {
	logo := m.renderLogo()
	message := m.theme.Muted.Render("observando a memória com calma")
	content := lipgloss.JoinVertical(lipgloss.Center, logo, "", message)
	return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center, content)
}
