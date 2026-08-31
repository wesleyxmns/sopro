package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wesleyxmns/sopro/internal/memory"
	processdomain "github.com/wesleyxmns/sopro/internal/process"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) View() string {
	if m.Width < 40 || m.Height < 12 {
		return "Sopro precisa de um terminal com pelo menos 40×12."
	}
	if m.ShowSplash {
		return m.renderSplash()
	}
	dashboard := m.renderDashboard()
	if m.Pending != nil {
		return m.renderConfirmationFocus(dashboard)
	}
	if m.PendingUpdate != nil {
		return m.renderUpdateConfirmationFocus(dashboard)
	}
	if m.Acting {
		return m.renderExecutionFocus(dashboard)
	}
	return dashboard
}

func (m Model) renderDashboard() string {
	layout := calculateLayout(m.Width, m.Height)
	header := m.renderDashboardHeader(layout)
	navigation := m.renderProcessSectionHeader(layout)
	tableHeader := m.renderTableHeader(layout.listWidth, layout.mode)
	list := tableHeader + "\n" + m.viewport.View()

	main := list
	if layout.mode == layoutWide {
		main = lipgloss.JoinHorizontal(
			lipgloss.Bottom,
			lipgloss.NewStyle().Width(layout.listWidth).Render(list),
			m.renderDetails(layout.detailWidth),
		)
	}

	footer := m.renderFooter(layout.width)
	return strings.Join([]string{header, navigation, main, footer}, "\n")
}

func (m Model) renderTopBar() string {
	brand := m.theme.Brand.Render(wordmark)
	if m.Width < 54 {
		return brand
	}
	right := m.theme.Muted.Render(m.Capabilities.Platform + " · " + m.privilegeLabel())
	indicatorWidth := m.Width - lipgloss.Width(brand) - lipgloss.Width(right) - 3
	if m.Width < 72 {
		indicatorWidth = m.Width - lipgloss.Width(brand) - 2
		right = ""
	}
	left := brand + "  " + m.renderOperationalIndicators(indicatorWidth)
	if right == "" {
		return ansi.Truncate(left, m.Width, "")
	}
	gap := max(m.Width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return ansi.Truncate(left+strings.Repeat(" ", gap)+right, m.Width, "")
}

func (m Model) renderWideContext(width int) string {
	right := m.theme.Muted.Render(m.Capabilities.Platform + " · " + m.privilegeLabel())
	brand := m.theme.Brand.Render("SOPRO")
	indicatorWidth := width - lipgloss.Width(brand) - lipgloss.Width(right) - 3
	left := brand + "  " + m.renderOperationalIndicators(indicatorWidth)
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return ansi.Truncate(brand+"  "+m.renderOperationalIndicators(width-lipgloss.Width(brand)-2), width, "")
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) renderOperationalIndicators(width int) string {
	pressure, pressureStyle := m.pressureStatus()
	telemetry, telemetryStyle := m.telemetryStatus()
	badge := ""
	if m.UpdateAvailable != nil {
		badge = m.theme.Warning.Render("≋ "+m.UpdateAvailable.TagName+" [U]") + m.theme.Divider.Render(" · ")
	}
	if width < 26 {
		return badge + pressureStyle.Render("●") + m.theme.Divider.Render(" · ") + telemetryStyle.Render("◌")
	}
	if width < 52 {
		return badge + m.theme.Muted.Render("mem ") + pressureStyle.Render("● normal") +
			m.theme.Divider.Render(" · ") + m.theme.Muted.Render("sync ") + telemetryStyle.Render("◌")
	}
	if width < 70 {
		return badge + m.theme.Muted.Render("memória ") + pressureStyle.Render("● "+pressure) +
			m.theme.Divider.Render(" · ") + m.theme.Muted.Render("coleta ") + telemetryStyle.Render(telemetry)
	}
	return badge + m.theme.Muted.Render("SAÚDE DA MEMÓRIA ") + pressureStyle.Render("● "+pressure) +
		m.theme.Divider.Render("   │   ") + m.theme.Muted.Render("COLETA DE DADOS ") + telemetryStyle.Render(telemetry)
}

func (m Model) telemetryStatus() (string, lipgloss.Style) {
	if m.Loading {
		return "◌ atualizando", m.theme.Focus
	}
	return "● em dia", m.theme.Good
}

func (m Model) pressureStatus() (string, lipgloss.Style) {
	pressure := m.Snapshot.Memory.Pressure
	if pressure.Supported {
		if pressure.Full.Avg10 >= 1 || pressure.Some.Avg10 >= 20 {
			return "PSI crítico", m.theme.Danger
		}
		if pressure.Full.Avg10 >= .1 || pressure.Some.Avg10 >= 5 {
			return "PSI elevado", m.theme.Warning
		}
		return "PSI normal", m.theme.Good
	}
	if m.Snapshot.Memory.Total > 0 {
		used := float64(m.Snapshot.Memory.Used) / float64(m.Snapshot.Memory.Total)
		if used >= .90 {
			return "uso crítico", m.theme.Danger
		}
		if used >= .75 {
			return "uso elevado", m.theme.Warning
		}
	}
	return "uso normal", m.theme.Good
}

func (m Model) privilegeLabel() string {
	if m.Capabilities.Elevated {
		return "sessão privilegiada"
	}
	return "sessão comum"
}

func (m Model) renderMemorySummary(width int) string {
	metrics := m.Snapshot.Memory
	percent := 0
	if metrics.Total > 0 {
		percent = int(float64(metrics.Used) / float64(metrics.Total) * 100)
	}
	if width < 72 {
		barWidth := max(min(width-15, 24), 10)
		filled := min(barWidth*percent/100, barWidth)
		bar := m.theme.Focus.Render(strings.Repeat("█", filled)) +
			m.theme.Divider.Render(strings.Repeat("░", barWidth-filled))
		primary := fmt.Sprintf("RAM [%s] %d%%", bar, percent)
		secondary := fmt.Sprintf(
			"%s / %s · disp %s",
			memory.FormatBytes(metrics.Used),
			memory.FormatBytes(metrics.Total),
			memory.FormatBytes(metrics.Available),
		)
		return primary + "\n" + m.theme.Muted.Render(secondary)
	}

	const gapWidth = 3
	usageWidth := max(min(width*45/100, 42), 30)
	gridWidth := max(width-usageWidth-gapWidth, 1)
	usage := m.renderMemoryUsage(percent, usageWidth)
	grid := m.renderMemoryGrid(gridWidth)
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		usage,
		strings.Repeat(" ", gapWidth),
		grid,
	)
}

func (m Model) renderMemoryUsage(percent, width int) string {
	metrics := m.Snapshot.Memory
	barWidth := max(width, 10)
	filled := min(barWidth*percent/100, barWidth)
	bar := m.theme.Focus.Render(strings.Repeat("█", filled)) +
		m.theme.Divider.Render(strings.Repeat("░", barWidth-filled))
	lines := []string{
		m.theme.Muted.Render("USO DA MEMÓRIA") + "  " + m.theme.Focus.Render(fmt.Sprintf("%d%%", percent)),
		bar,
		m.theme.Strong.Render(fmt.Sprintf(
			"%s de %s",
			memory.FormatBytes(metrics.Used),
			memory.FormatBytes(metrics.Total),
		)),
		m.renderMemoryTrend(width),
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func (m Model) renderMemoryTrend(width int) string {
	const levels = "▁▂▃▄▅▆▇█"
	if len(m.memoryHistory) == 0 {
		return m.theme.Muted.Render("HISTÓRICO  aguardando amostras")
	}
	maxPoints := max(width-11, 1)
	start := max(len(m.memoryHistory)-maxPoints, 0)
	var trend strings.Builder
	for _, snapshot := range m.memoryHistory[start:] {
		percent := 0
		if snapshot.Total > 0 {
			percent = min(int(float64(snapshot.Used)/float64(snapshot.Total)*100), 100)
		}
		index := min(percent*(len([]rune(levels))-1)/100, len([]rune(levels))-1)
		trend.WriteRune([]rune(levels)[index])
	}
	return m.theme.Muted.Render("HISTÓRICO  ") + m.theme.Focus.Render(trend.String())
}

func (m Model) renderMemoryGrid(width int) string {
	metrics := m.Snapshot.Memory
	contentWidth := max(width-2, 3)
	baseWidth := max(contentWidth/3, 1)
	widths := []int{baseWidth, baseWidth, max(contentWidth-baseWidth*2, 1)}
	separator := m.theme.Divider.Render("│")

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderMetricCell("DISPONÍVEL", memory.FormatBytes(metrics.Available), "livre p/ uso", widths[0]),
		separator,
		m.renderMetricCell("RECUPERÁVEL", memory.FormatBytes(metrics.Reclaimable), "cache "+memory.FormatBytes(metrics.Cache), widths[1]),
		separator,
		m.renderMetricCell("SWAP", memory.FormatBytes(metrics.SwapUsed), "de "+memory.FormatBytes(metrics.SwapTotal), widths[2]),
	)
}

func (m Model) renderMetricCell(label, value, note string, width int) string {
	lines := []string{
		m.theme.Muted.Render(ansi.Truncate(label, width, "…")),
		m.theme.Strong.Render(ansi.Truncate(value, width, "…")),
		m.theme.Muted.Render(ansi.Truncate(note, width, "…")),
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func (m Model) renderProcessSectionHeader(layout layout) string {
	left := m.theme.Focus.Render("Processos")
	if m.Searching || m.query.Search != "" {
		left += m.theme.Focus.Render("  busca: " + m.query.Search + "▌")
	}
	line := left
	if layout.mode != layoutCompact {
		obs := fmt.Sprintf("%d de %d em observação", len(m.Snapshot.Processes), len(m.allProcesses))
		right := m.theme.Muted.Render(obs + " · " + strings.ToUpper(sortLabel(m.query.Sort)) + " · " + strings.ToUpper(groupLabel(m.groupMode)))
		gap := max(layout.width-lipgloss.Width(left)-lipgloss.Width(right), 1)
		line = ansi.Truncate(left+strings.Repeat(" ", gap)+right, layout.width, "")
	}
	tabs := ansi.Truncate(m.renderCategoryTabs(layout.width), layout.width, "")
	return m.theme.Divider.Render(strings.Repeat("─", layout.width)) + "\n" + line + "\n" + tabs + "\n"
}

func (m Model) renderCategoryTabs(width int) string {
	counts := make(map[processdomain.Category]int)
	for _, proc := range m.allProcesses {
		counts[proc.Category]++
	}

	tabs := []struct {
		category processdomain.Category
		label    string
		key      string
	}{
		{"", "Todos", "1"},
		{processdomain.CategoryBrowser, "Navegadores", "2"},
		{processdomain.CategoryContainer, "Containers", "3"},
		{processdomain.CategoryDevelopment, "Dev", "4"},
		{processdomain.CategoryDatabase, "Bancos", "5"},
		{processdomain.CategoryJVM, "JVM", "6"},
		{processdomain.CategorySystem, "Sistema", "7"},
		{processdomain.CategoryOther, "Outros", "8"},
	}

	var renderedTabs []string
	for _, tab := range tabs {
		count := len(m.allProcesses)
		if tab.category != "" {
			count = counts[tab.category]
		}
		text := fmt.Sprintf(" %s:%s (%d) ", tab.key, tab.label, count)
		if m.query.Category == tab.category {
			renderedTabs = append(renderedTabs, m.theme.Selected.Render(text))
		} else {
			renderedTabs = append(renderedTabs, m.theme.Muted.Render(text))
		}
	}

	return strings.Join(renderedTabs, " ")
}

func (m Model) renderDashboardHeader(layout layout) string {
	status := ""
	if m.Message != "" {
		status = m.renderStatus(m.Message)
	}
	if layout.mode != layoutWide {
		lines := []string{m.renderTopBar()}
		if status != "" {
			lines = append(lines, status)
		}
		lines = append(lines, m.renderMemorySummary(layout.width))
		return strings.Join(lines, "\n")
	}

	logo := m.renderLogo()
	const gap = 3
	rightWidth := max(layout.width-lipgloss.Width(logo)-gap, 1)
	rightLines := []string{m.renderWideContext(rightWidth)}
	if status != "" {
		rightLines = append(rightLines, status)
	}
	rightLines = append(rightLines, m.renderMemorySummary(rightWidth))
	right := strings.Join(rightLines, "\n")

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		logo,
		strings.Repeat(" ", gap),
		lipgloss.NewStyle().Width(rightWidth).Render(right),
	)
}

func (m Model) renderTableHeader(width int, mode layoutMode) string {
	var header string
	if mode == layoutCompact {
		header = fmt.Sprintf("  %-7s %9s %s", "PID", "MEMÓRIA", "PROCESSO")
	} else {
		header = fmt.Sprintf("  %-7s %-14s %9s %-6s %-10s %s", "PID", "USUÁRIO", "MEMÓRIA", "  CPU", "ESTADO", "PROCESSO")
	}
	return m.theme.Muted.Render(ansi.Truncate(header, width, ""))
}

func (m Model) renderProcessRows(width int, mode layoutMode) string {
	rows := make([]string, 0, len(m.Snapshot.Processes))
	for index, proc := range m.Snapshot.Processes {
		marker := "·"
		if index == m.Cursor {
			marker = "▌"
		}

		var row string
		command := proc.Command
		if proc.ContainerName != "" {
			command = proc.ContainerName
		}
		if m.groupMode == groupCategory {
			command = categoryLabel(proc.Category) + " › " + command
		}
		if m.groupMode == groupTree {
			command = strings.Repeat("  ", m.processDepth[proc.Identity]) + "└ " + command
		}

		pidStr := strconv.Itoa(int(proc.PID))
		if proc.PID <= 0 {
			pidStr = "-"
		}
		userStr := proc.User
		if userStr == "" {
			userStr = "-"
		}

		if mode == layoutCompact {
			commandWidth := max(width-20, 1)
			row = fmt.Sprintf(
				"%s %-7s %9s %s",
				marker,
				pidStr,
				memory.FormatBytes(proc.MemoryBytes),
				ansi.Truncate(command, commandWidth, "…"),
			)
		} else {
			cpuStr := fmt.Sprintf("%5.1f%%", proc.CPUPct)
			if proc.State == processdomain.StateStopped {
				cpuStr = "     -"
			}
			commandWidth := max(width-53, 1)
			row = fmt.Sprintf(
				"%s %-7s %-14s %9s %6s %s %s",
				marker,
				pidStr,
				ansi.Truncate(userStr, 14, "…"),
				memory.FormatBytes(proc.MemoryBytes),
				cpuStr,
				m.renderStateLabel(proc),
				ansi.Truncate(command, commandWidth, "…"),
			)
		}
		row = ansi.Truncate(row, max(width-1, 1), "")
		if index == m.Cursor {
			row = m.theme.Selected.Render(row)
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return m.theme.Muted.Render("Nenhum processo disponível.")
	}
	return strings.Join(rows, "\n")
}

func (m Model) renderDetails(width int) string {
	proc, ok := m.selectedProcess()
	if !ok {
		return ""
	}
	innerWidth := max(width-4, 1)
	title := proc.Command
	if proc.ContainerName != "" {
		title = fmt.Sprintf("%s (%s)", proc.ContainerName, proc.Command)
	}
	command := ansi.Truncate(title, innerWidth, "…")
	identity := ansi.Truncate(fmt.Sprintf("PID %d · %s · %s", proc.PID, proc.User, categoryLabel(proc.Category)), innerWidth, "…")
	separator := m.theme.Divider.Render(strings.Repeat("─", innerWidth))
	content := []string{
		m.theme.Muted.Render("PROCESSO SELECIONADO"),
		m.theme.Strong.Render(command),
		m.theme.Muted.Render(identity),
	}
	if proc.ImageName != "" {
		content = append(content, m.theme.Muted.Render("IMAGEM    ")+m.theme.Focus.Render(ansi.Truncate(proc.ImageName, max(innerWidth-10, 1), "…")))
	}
	if proc.CommandLine != "" && proc.CommandLine != proc.Command {
		content = append(content, m.theme.Muted.Render("COMANDO   ")+m.theme.Focus.Render(ansi.Truncate(proc.CommandLine, max(innerWidth-10, 1), "…")))
	}
	if len(proc.Contexts) > 0 {
		tags := make([]string, 0, len(proc.Contexts))
		for _, c := range proc.Contexts {
			tags = append(tags, string(c))
		}
		contextLine := m.theme.Muted.Render("CONTEXTO  ") + m.theme.Focus.Render(ansi.Truncate(strings.Join(tags, " · "), max(innerWidth-10, 1), "…"))
		content = append(content, contextLine)
	}
	content = append(content,
		separator,
		m.theme.Muted.Render("RECURSOS E ESTADO"),
		m.renderProcessMetricGrid(proc, innerWidth),
		m.renderLeakAssessment(proc, innerWidth),
		separator,
		m.theme.Muted.Render("AÇÕES"),
		m.renderPanelActions(proc, innerWidth),
	)
	return lipgloss.NewStyle().
		Width(innerWidth).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.MutedColor).
		Render(strings.Join(content, "\n"))
}

func (m Model) renderLeakAssessment(proc processdomain.Info, width int) string {
	assessment := proc.Leak
	label := "coletando amostras"
	style := m.theme.Muted
	if assessment.Status == processdomain.LeakStable {
		label = "estável"
		style = m.theme.Good
	}
	if assessment.Status == processdomain.LeakSuspected {
		rate := memory.FormatBytes(uint64(assessment.GrowthBytesPerSecond * 60))
		label = fmt.Sprintf("suspeito (+%s/min)", rate)
		style = m.theme.Warning
	}
	return m.theme.Muted.Render("LEAK GUARD  ") + style.Render(ansi.Truncate(label, max(width-12, 1), "…"))
}

func filterLabel(category processdomain.Category) string {
	if category == "" {
		return "todos"
	}
	return categoryLabel(category)
}

func sortLabel(field processdomain.SortField) string {
	switch field {
	case processdomain.SortCPU:
		return "ordenado por CPU ↓"
	case processdomain.SortCommand:
		return "ordenado por nome ↑"
	default:
		return "ordenado por memória ↓"
	}
}

func groupLabel(mode processGroupMode) string {
	switch mode {
	case groupCategory:
		return "categorias"
	case groupTree:
		return "árvore"
	default:
		return "lista"
	}
}

func (m Model) renderProcessMetricGrid(proc processdomain.Info, width int) string {
	contentWidth := max(width-3, 4)
	baseWidth := max(contentWidth/4, 1)
	widths := []int{baseWidth, baseWidth, baseWidth, max(contentWidth-baseWidth*3, 1)}
	separator := m.theme.Divider.Render("│")
	cpuText := fmt.Sprintf("%.1f%%", proc.CPUPct)
	if proc.State == processdomain.StateStopped {
		cpuText = "-"
	}
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderProcessMetric("MEMÓRIA", memory.FormatBytes(proc.MemoryBytes), widths[0]),
		separator,
		m.renderProcessMetric("CPU", cpuText, widths[1]),
		separator,
		m.renderProcessMetric("ESTADO", m.renderStateLabel(proc), widths[2]),
		separator,
		m.renderProcessMetric("RISCO", riskLabel(proc.Risk), widths[3]),
	)
}

func (m Model) renderProcessMetric(label, value string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(strings.Join([]string{
		m.theme.Muted.Render(ansi.Truncate(label, width, "…")),
		m.theme.Strong.Render(ansi.Truncate(value, width, "…")),
	}, "\n"))
}

func (m Model) renderPanelActions(proc processdomain.Info, width int) string {
	if proc.State == processdomain.StateStopped {
		actionLine := m.renderKeyHint(keyHint{"s", "docker start"})
		return ansi.Truncate(actionLine, width, "")
	}

	pauseLabel := "pausar"
	if proc.State == processdomain.StatePaused {
		pauseLabel = "retomar"
	}
	columnWidth := max((width-2)/2, 1)
	first := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(columnWidth).Render(m.renderKeyHint(keyHint{"p", pauseLabel})),
		"  ",
		m.renderKeyHint(keyHint{"x", "encerrar"}),
	)
	second := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(columnWidth).Render(m.renderKeyHint(keyHint{"k", "forçar"})),
		"  ",
		m.renderKeyHint(keyHint{"c", "cache"}),
	)
	lines := []string{ansi.Truncate(first, width, ""), ansi.Truncate(second, width, "")}

	if proc.Category == processdomain.CategoryContainer || proc.ContainerID != "" {
		third := lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.NewStyle().Width(columnWidth).Render(m.renderKeyHint(keyHint{"d", "docker stop"})),
			"  ",
			m.renderKeyHint(keyHint{"r", "docker restart"}),
		)
		fourth := m.renderKeyHint(keyHint{"z", "docker pause"})
		lines = append(lines, ansi.Truncate(third, width, ""), ansi.Truncate(fourth, width, ""))
	} else if proc.Category == processdomain.CategoryBrowser || hasContextTag(proc.Contexts, processdomain.ContextBrowserDebug) {
		third := lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.NewStyle().Width(columnWidth).Render(m.renderKeyHint(keyHint{"b", "fechar vazias"})),
			"  ",
			m.renderKeyHint(keyHint{"u", "suspender abas"}),
		)
		lines = append(lines, ansi.Truncate(third, width, ""))
	} else if proc.Category == processdomain.CategoryJVM || hasContextTag(proc.Contexts, processdomain.ContextTag("jvm-runtime")) {
		lines = append(lines, ansi.Truncate(m.renderKeyHint(keyHint{"j", "forçar GC"}), width, ""))
	} else if proc.Category == processdomain.CategoryDevelopment || hasContextTag(proc.Contexts, processdomain.ContextGitRepository) {
		third := lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.NewStyle().Width(columnWidth).Render(m.renderKeyHint(keyHint{"w", "git status"})),
			"  ",
			m.renderKeyHint(keyHint{"v", "git fetch"}),
		)
		lines = append(lines, ansi.Truncate(third, width, ""))
	}

	return strings.Join(lines, "\n")
}

func hasContextTag(contexts []processdomain.ContextTag, target processdomain.ContextTag) bool {
	for _, c := range contexts {
		if c == target {
			return true
		}
	}
	return false
}

func (m Model) renderFooter(width int) string {
	hints := hintsForWidth(width, m.Pending != nil)
	parts := make([]string, 0, len(hints))
	for _, hint := range hints {
		parts = append(parts, m.renderKeyHint(hint))
	}
	separator := m.theme.Divider.Render(" · ")
	line := ansi.Truncate(strings.Join(parts, separator), width, "")
	return m.theme.Divider.Render(strings.Repeat("─", width)) + "\n" + line
}

func (m Model) renderKeyHint(hint keyHint) string {
	return m.theme.Focus.Render("["+hint.key+"]") + " " + m.theme.Muted.Render(hint.label)
}

func (m Model) renderStatus(status string) string {
	lower := strings.ToLower(status)
	if strings.Contains(lower, "falh") || strings.Contains(lower, "indisponível") {
		return m.theme.Danger.Render(status)
	}
	if m.Pending != nil {
		return m.theme.Warning.Render(status)
	}
	return m.theme.Muted.Render(status)
}

func (m Model) renderStateLabel(proc processdomain.Info) string {
	switch proc.State {
	case processdomain.StatePaused:
		return m.theme.Warning.Render("Ⅱ") + " pausado "
	case processdomain.StateRunning:
		return m.theme.Good.Render("●") + " ativo   "
	case processdomain.StateStopped:
		return m.theme.Muted.Render("○ parado  ")
	default:
		return m.theme.Muted.Render("? desconh.")
	}
}

func stateLabel(proc processdomain.Info) string {
	switch proc.State {
	case processdomain.StatePaused:
		return "Ⅱ pausado"
	case processdomain.StateRunning:
		return "● ativo"
	case processdomain.StateStopped:
		return "○ parado"
	default:
		return "? desconhec."
	}
}

func categoryLabel(category processdomain.Category) string {
	switch category {
	case processdomain.CategorySystem:
		return "sistema"
	case processdomain.CategoryContainer:
		return "container"
	case processdomain.CategoryBrowser:
		return "navegador"
	case processdomain.CategoryDatabase:
		return "banco de dados"
	case processdomain.CategoryDevelopment:
		return "desenvolvimento"
	case processdomain.CategoryJVM:
		return "JVM"
	default:
		return "outro"
	}
}
