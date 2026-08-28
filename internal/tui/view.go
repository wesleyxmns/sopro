package tui

import (
	"fmt"
	"strings"
	"memcleaner/internal/system"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.Width < 20 || m.Height < 16 {
		return "Terminal too small..."
	}

	// Cabeçalho Principal (Métricas Globais)
	var ramProgress int
	if m.Metrics.TotalRAM > 0 {
		ramProgress = int(float64(m.Metrics.UsedRAM) / float64(m.Metrics.TotalRAM) * 20)
	}
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
	maxVisibleRows := m.Height - 16
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
		memStr := fmt.Sprintf("%.1f%%", p.MemPct)
		cpuStr := fmt.Sprintf("%.1f%%", p.CPUPct)

		if m.Width > 80 {
			cmdWidth := m.Width - 8 - 42 // width - paddings - other columns
			cmd := p.Command
			if cmdWidth > 3 && len(cmd) > cmdWidth {
				cmd = cmd[:cmdWidth-3] + "..."
			} else if cmdWidth > 0 && len(cmd) > cmdWidth {
				cmd = cmd[:cmdWidth]
			}
			line = fmt.Sprintf("%s %-8d %-12s %-8s %-8s %s", cursor, p.PID, p.User, memStr, cpuStr, cmd)
		} else {
			cmdWidth := m.Width - 8 - 20
			cmd := p.Command
			if cmdWidth > 3 && len(cmd) > cmdWidth {
				cmd = cmd[:cmdWidth-3] + "..."
			} else if cmdWidth > 0 && len(cmd) > cmdWidth {
				cmd = cmd[:cmdWidth]
			}
			line = fmt.Sprintf("%s %-8d %-8s %s", cursor, p.PID, memStr, cmd)
		}
		processLines = append(processLines, style.Render(line))
	}

	sepWidth := m.Width - 8
	if sepWidth < 0 { sepWidth = 0 }
	
	tableContent := tableHeader + "\n" + strings.Repeat("─", sepWidth) + "\n" + strings.Join(processLines, "\n")
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
