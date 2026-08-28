package tui

import (
	"fmt"
	"strings"
	"memcleaner/internal/system"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.Width == 0 || m.Height == 0 {
		return "Rendering Sopro..."
	}

	// Cabeçalho Principal (Métricas Globais)
	ramProgress := int(float64(m.Metrics.UsedRAM) / float64(m.Metrics.TotalRAM) * 20)
	if ramProgress < 0 { ramProgress = 0 }
	if ramProgress > 20 { ramProgress = 20 }
	progressBar := SoproDS.Safe.Render(strings.Repeat("█", ramProgress)) + SoproDS.Muted.Render(strings.Repeat("░", 20-ramProgress))

	headerContent := fmt.Sprintf("SOPRO [Root: %v] | %s\n\nRAM:  [%s] %s / %s (Cache: %s)", 
		m.IsRoot, 
		SoproDS.Warning.Render(m.Message), 
		progressBar,
		system.FormatBytes(m.Metrics.UsedRAM),
		system.FormatBytes(m.Metrics.TotalRAM),
		system.FormatBytes(m.Metrics.CacheRAM),
	)

	headerBox := lipgloss.NewStyle().
		Border(SoproDS.Border).
		BorderForeground(lipgloss.Color("#7AA2F7")).
		Width(m.Width - 4).
		Padding(1).
		Render(headerContent)

	// Layout Adaptativo da Tabela de Processos
	var tableHeader string
	if m.Width > 80 {
		tableHeader = fmt.Sprintf("  %-8s %-12s %-8s %-8s %s", "PID", "USER", "MEM%", "CPU%", "COMMAND")
	} else {
		// Truncamento adaptativo para telas pequenas (oculta USER e CPU% para não quebrar layout)
		tableHeader = fmt.Sprintf("  %-8s %-8s %s", "PID", "MEM%", "COMMAND")
	}

	var processLines []string
	// Viewport Scroll: Ajusta exibição para caber na janela
	maxVisibleRows := m.Height - 12
	if maxVisibleRows < 1 { maxVisibleRows = 1 }

	startRow := m.Cursor - (maxVisibleRows / 2)
	if startRow < 0 { startRow = 0 }
	endRow := startRow + maxVisibleRows
	if endRow > len(m.Processes) {
		endRow = len(m.Processes)
		startRow = endRow - maxVisibleRows
		if startRow < 0 { startRow = 0 }
	}

	for i := startRow; i < endRow; i++ {
		p := m.Processes[i]
		cursor := " "
		style := lipgloss.NewStyle()
		if m.Cursor == i {
			cursor = "❯"
			style = SoproDS.Active
		}

		var line string
		if m.Width > 80 {
			line = fmt.Sprintf("%s %-8d %-12s %-8.1f%% %-8.1f%% %s", cursor, p.PID, p.User, p.MemPct, p.CPUPct, p.Command)
		} else {
			line = fmt.Sprintf("%s %-8d %-8.1f%% %s", cursor, p.PID, p.MemPct, p.Command)
		}
		processLines = append(processLines, style.Render(line))
	}

	tableContent := tableHeader + "\n" + strings.Repeat("─", m.Width-6) + "\n" + strings.Join(processLines, "\n")
	tableBox := lipgloss.NewStyle().
		Border(SoproDS.Border).
		BorderForeground(lipgloss.Color("#565F89")).
		Width(m.Width - 4).
		Height(maxVisibleRows + 3).
		Padding(1, 2).
		Render(tableContent)

	// Rodapé de atalhos
	footer := SoproDS.Muted.Render("[↑/↓] Navegar  [k] Kill Processo  [c] Limpar Cache  [q] Sair")

	return headerBox + "\n" + tableBox + "\n" + footer
}
