package tui

import (
	"fmt"
	"strings"

	"github.com/wesleyxmns/sopro/internal/control"
	"github.com/wesleyxmns/sopro/internal/memory"
	processdomain "github.com/wesleyxmns/sopro/internal/process"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) renderConfirmationFocus(background string) string {
	request := m.Pending
	if request == nil {
		return ""
	}

	title, description := actionCopy(request.Action)
	modalWidth := min(max(m.Width-10, 34), 64)
	innerWidth := max(modalWidth-6, 20)
	accent := m.theme.Warning
	marker := "▲"
	if request.Action == control.ActionKill {
		accent = m.theme.Danger
		marker = "!"
	}

	lines := []string{
		m.theme.Brand.Render(wordmark) + "  " + accent.Render(marker+" CONFIRMAR AÇÃO"),
		m.theme.Divider.Render(strings.Repeat("─", innerWidth)),
		m.theme.Strong.Render(title),
		description,
	}

	if request.Action == control.ActionClean {
		lines = append(lines,
			"",
			m.theme.Muted.Render("Alvo"),
			"Cache de páginas, dentries e inodes",
			m.theme.Warning.Render("Pode aumentar I/O e uso de CPU temporariamente."),
		)
	} else if target, ok := m.processByIdentity(request.Process); ok {
		lines = append(lines,
			"",
			m.theme.Muted.Render("Processo"),
			m.theme.Strong.Render(ansi.Truncate(target.Command, innerWidth, "…")),
			fmt.Sprintf("PID %-8d  usuário %s", target.PID, ansi.Truncate(target.User, max(innerWidth-22, 1), "…")),
			fmt.Sprintf("memória %-10s  CPU %.1f%%", memory.FormatBytes(target.MemoryBytes), target.CPUPct),
			fmt.Sprintf("estado %-12s risco %s", stateLabel(target), riskLabel(target.Risk)),
		)
	}

	if request.Action == control.ActionKill {
		lines = append(lines, "", m.theme.Danger.Render("Esta ação é imediata e não permite recuperação."))
	}
	lines = append(lines,
		"",
		accent.Render("[ enter / y ] CONFIRMAR")+"    "+m.theme.Muted.Render("[ esc / n ] cancelar"),
	)

	panelStyle := lipgloss.NewStyle().
		Width(innerWidth).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent.GetForeground())
	panel := panelStyle.Render(strings.Join(lines, "\n"))
	return overlayCentered(background, panel, m.Width, m.Height)
}

func (m Model) renderUpdateConfirmationFocus(background string) string {
	rel := m.PendingUpdate
	if rel == nil {
		return ""
	}

	modalWidth := min(max(m.Width-10, 34), 64)
	innerWidth := max(modalWidth-6, 20)
	accent := m.theme.Focus

	lines := []string{
		m.theme.Brand.Render(wordmark) + "  " + accent.Render("≋ ATUALIZAÇÃO DISPONÍVEL"),
		m.theme.Divider.Render(strings.Repeat("─", innerWidth)),
		m.theme.Strong.Render("ATUALIZAR O SOPRO"),
		"Baixa e instala a nova versão oficial diretamente do GitHub.",
		"",
		m.theme.Muted.Render("Versão disponível"),
		fmt.Sprintf("%s (publicado em %s)", m.theme.Good.Render(rel.TagName), m.theme.Muted.Render(rel.PublishedAt.Format("02/01/2006"))),
		"",
		accent.Render("[ enter / y ] ATUALIZAR") + "    " + m.theme.Muted.Render("[ esc / n ] cancelar"),
	}

	panelStyle := lipgloss.NewStyle().
		Width(innerWidth).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent.GetForeground())
	panel := panelStyle.Render(strings.Join(lines, "\n"))
	return overlayCentered(background, panel, m.Width, m.Height)
}

func (m Model) renderExecutionFocus(background string) string {
	action := control.Action("")
	if m.ActiveAction != nil {
		action = m.ActiveAction.Action
	}
	title, _ := actionCopy(action)
	if title == "" {
		title = "EXECUTANDO AÇÃO"
	}
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		m.theme.Brand.Render(wordmark),
		"",
		m.theme.Focus.Render("◌ "+title),
		m.theme.Muted.Render("revalidando identidade e aplicando a ação…"),
		"",
		m.theme.Muted.Render("q sair"),
	)
	panel := lipgloss.NewStyle().
		Padding(1, 3).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Focus.GetForeground()).
		Render(content)
	return overlayCentered(background, panel, m.Width, m.Height)
}

func (m Model) processByIdentity(identity processdomain.Identity) (processdomain.Info, bool) {
	for _, candidate := range m.Snapshot.Processes {
		if candidate.Identity == identity {
			return candidate, true
		}
	}
	return processdomain.Info{}, false
}

func actionCopy(action control.Action) (string, string) {
	switch action {
	case control.ActionTerminate:
		return "ENCERRAR PROCESSO", "Solicita que o processo finalize de forma organizada."
	case control.ActionKill:
		return "FORÇAR ENCERRAMENTO", "Interrompe o processo sem permitir limpeza ou salvamento."
	case control.ActionPause:
		return "PAUSAR PROCESSO", "Suspende a execução até que o processo seja retomado."
	case control.ActionResume:
		return "RETOMAR PROCESSO", "Permite que o processo pausado continue a execução."
	case control.ActionClean:
		return "LIMPAR CACHE DO SISTEMA", "Solicita ao kernel a liberação manual de caches recuperáveis."
	default:
		return "", ""
	}
}

func riskLabel(risk processdomain.Risk) string {
	switch risk {
	case processdomain.RiskCritical:
		return "CRÍTICO"
	case processdomain.RiskWarning:
		return "ATENÇÃO"
	default:
		return "normal"
	}
}
