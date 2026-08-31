package tui

import (
	"strings"
	"testing"

	processdomain "github.com/wesleyxmns/sopro/internal/process"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestViewDoesNotOverflowResponsiveWidth(t *testing.T) {
	for _, size := range []tea.WindowSizeMsg{
		{Width: 40, Height: 12},
		{Width: 60, Height: 20},
		{Width: 90, Height: 24},
		{Width: 120, Height: 30},
	} {
		model, backend := newTestModel()
		model.applySnapshot(backend.snapshot)
		updated, _ := model.Update(size)
		view := updated.(Model).View()
		for lineNumber, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got > size.Width {
				t.Fatalf("layout %dx%d line %d width = %d; content %q", size.Width, size.Height, lineNumber+1, got, line)
			}
		}
	}
}

func TestSplashShowsBrandBeforeDashboard(t *testing.T) {
	model, _ := newTestModel()
	model.ShowSplash = true
	view := model.View()
	if !strings.Contains(view, "____") || !strings.Contains(view, "observando a memória com calma") {
		t.Fatal("splash did not render the Sopro brand")
	}
	if strings.Contains(view, "Processos") {
		t.Fatal("dashboard rendered underneath the splash")
	}

	updated, _ := model.Update(splashFinishedMsg{})
	if updated.(Model).ShowSplash {
		t.Fatal("splash completion did not reveal the dashboard")
	}
}

func TestSplashDoesNotOverflowResponsiveWidth(t *testing.T) {
	for _, width := range []int{40, 60, 100} {
		model, _ := newTestModel()
		model.Width = width
		model.Height = 16
		model.ShowSplash = true
		for lineNumber, line := range strings.Split(model.View(), "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("splash width %d line %d = %d; content %q", width, lineNumber+1, got, line)
			}
		}
	}
}

func TestConfirmationOverlaysDashboard(t *testing.T) {
	model, backend := newTestModel()
	model.applySnapshot(backend.snapshot)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model = updated.(Model)
	view := model.View()

	for _, expected := range []string{
		"CONFIRMAR AÇÃO",
		"FORÇAR ENCERRAMENTO",
		"proc1",
		"PID 100",
		"[ enter / y ] CONFIRMAR",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("confirmation view omitted %q", expected)
		}
	}
	if !strings.Contains(view, "Processos") {
		t.Fatal("confirmation did not preserve the dashboard in the background")
	}
}

func TestConfirmationDoesNotOverflowResponsiveWidth(t *testing.T) {
	for _, width := range []int{40, 60, 100} {
		model, backend := newTestModel()
		model.Width = width
		model.Height = 24
		model.applySnapshot(backend.snapshot)
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
		view := updated.(Model).View()
		for lineNumber, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("confirmation width %d line %d = %d; content %q", width, lineNumber+1, got, line)
			}
		}
	}
}

func TestConfirmedActionShowsExecutionFocus(t *testing.T) {
	model, backend := newTestModel()
	model.applySnapshot(backend.snapshot)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	model = updated.(Model)
	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if command == nil {
		t.Fatal("confirmed action did not start")
	}
	view := model.View()
	if !strings.Contains(view, "ENCERRAR PROCESSO") || !strings.Contains(view, "revalidando identidade") {
		t.Fatal("execution focus did not explain the active operation")
	}
	if !strings.Contains(view, "Processos") {
		t.Fatal("execution overlay did not preserve the dashboard in the background")
	}
}

func TestWideDashboardKeepsLargeLogoVisible(t *testing.T) {
	model, backend := newTestModel()
	model.Width = 150
	model.Height = 30
	model.ShowSplash = false
	model.applySnapshot(backend.snapshot)

	view := model.View()
	if !strings.Contains(view, "____") || !strings.Contains(view, "Processos") {
		t.Fatal("wide dashboard omitted the persistent large logo or process list")
	}
}

func TestCompactDashboardUsesCompactWordmark(t *testing.T) {
	model, backend := newTestModel()
	model.Width = 60
	model.Height = 20
	model.ShowSplash = false
	model.applySnapshot(backend.snapshot)

	view := model.View()
	if !strings.Contains(view, "SOPRO") {
		t.Fatal("compact dashboard omitted the wordmark")
	}
	if strings.Contains(view, "____") {
		t.Fatal("compact dashboard rendered the large logo")
	}
}

func TestOperationalIndicatorsShareDashboardHeader(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model, backend := newTestModel()
	model.theme = NewTheme()
	model.Width = 150
	model.Height = 30
	model.ShowSplash = false
	model.Loading = true
	model.applySnapshot(backend.snapshot)

	view := model.View()
	for _, line := range strings.Split(view, "\n") {
		memoryHealth := strings.Index(line, "SAÚDE DA MEMÓRIA")
		collection := strings.Index(line, "COLETA DE DADOS")
		if memoryHealth >= 0 && collection > memoryHealth && strings.Contains(line, "atualizando") {
			return
		}
	}
	t.Fatal("memory health and data collection indicators did not share the header")
}

func TestMediumHeaderUsesCondensedOperationalIndicators(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model, backend := newTestModel()
	model.theme = NewTheme()
	model.Width = 90
	model.Height = 24
	model.ShowSplash = false
	model.applySnapshot(backend.snapshot)

	view := model.View()
	if !strings.Contains(view, "memória ") || !strings.Contains(view, "coleta ") {
		t.Fatal("medium header omitted condensed operational indicators")
	}
}

func TestProcessListCommunicatesSelectionAndOrdering(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model, backend := newTestModel()
	model.theme = NewTheme()
	model.Width = 90
	model.Height = 24
	model.ShowSplash = false
	model.applySnapshot(backend.snapshot)

	view := model.View()
	for _, expected := range []string{"Processos", "em observação", "ORDENADO POR MEMÓRIA ↓", "▌"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("process list omitted visual cue %q", expected)
		}
	}
}

func TestWideDetailsCardGroupsContextAndActions(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model, backend := newTestModel()
	model.theme = NewTheme()
	model.Width = 120
	model.Height = 30
	model.ShowSplash = false
	model.applySnapshot(backend.snapshot)

	view := model.View()
	for _, expected := range []string{
		"PROCESSO SELECIONADO",
		"RECURSOS E ESTADO",
		"AÇÕES",
		"[p] pausar",
		"[x] encerrar",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("details card omitted %q", expected)
		}
	}
}

func TestWideDetailsCardUsesHorizontalMetricGrid(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model, backend := newTestModel()
	model.theme = NewTheme()
	model.Width = 120
	model.Height = 30
	model.ShowSplash = false
	model.applySnapshot(backend.snapshot)

	view := model.View()
	for _, line := range strings.Split(view, "\n") {
		memoryColumn := strings.Index(line, "MEMÓRIA")
		cpuColumn := strings.Index(line, "CPU")
		stateColumn := strings.Index(line, "ESTADO")
		riskColumn := strings.Index(line, "RISCO")
		if memoryColumn >= 0 && cpuColumn > memoryColumn && stateColumn > cpuColumn && riskColumn > stateColumn {
			return
		}
	}
	t.Fatal("details card metrics were not arranged in a horizontal grid")
}

func TestWideDetailsCardIsAnchoredToBottomRight(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model, backend := newTestModel()
	model.theme = NewTheme()
	model.Width = 120
	model.Height = 30
	model.ShowSplash = false
	model.applySnapshot(backend.snapshot)

	lines := strings.Split(model.View(), "\n")
	if len(lines) < 3 || !strings.Contains(lines[len(lines)-3], "╰") {
		t.Fatal("details card bottom border is not aligned with the bottom of the process area")
	}
	if lipgloss.Width(lines[len(lines)-3]) < 110 {
		t.Fatal("details card is not positioned on the right edge")
	}
}

func TestFooterStylesKeysSeparatelyFromLabels(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model, _ := newTestModel()
	model.theme = NewTheme()

	footer := model.renderFooter(120)
	for _, expected := range []string{"[↑↓] navegar", "[p] pausar/retomar", "[c] limpar cache", "[q] sair"} {
		if !strings.Contains(footer, expected) {
			t.Fatalf("footer omitted hint %q", expected)
		}
	}
	if !strings.Contains(footer, "·") {
		t.Fatal("footer omitted visual separators between commands")
	}
}

func TestMemorySummaryPlacesSecondaryMetricsInRightGrid(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model, backend := newTestModel()
	model.theme = NewTheme()
	model.Width = 90
	model.Height = 24
	model.ShowSplash = false
	model.applySnapshot(backend.snapshot)

	view := model.View()
	for _, line := range strings.Split(view, "\n") {
		available := strings.Index(line, "DISPONÍVEL")
		cache := strings.Index(line, "RECUPERÁVEL")
		swap := strings.Index(line, "SWAP")
		if available >= 0 && cache > available && swap > cache {
			return
		}
	}
	t.Fatal("secondary memory metrics were not arranged as an ordered right-side grid")
}

func TestCompactMemorySummaryRemainsCondensed(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model, backend := newTestModel()
	model.theme = NewTheme()
	model.Width = 60
	model.Height = 20
	model.ShowSplash = false
	model.applySnapshot(backend.snapshot)

	view := model.View()
	if !strings.Contains(view, "disp ") {
		t.Fatal("compact summary omitted available memory")
	}
	if strings.Contains(view, "DISPONÍVEL") || strings.Contains(view, "RECUPERÁVEL") || strings.Contains(view, "SWAP") {
		t.Fatal("compact summary rendered the expanded metric grid")
	}
}

func TestOverlayCenteredPreservesUncoveredBackground(t *testing.T) {
	background := strings.Join([]string{
		"abcdefghijkl",
		"mnopqrstuvwx",
		"yz0123456789",
		"ABCDEFGHIJKL",
	}, "\n")
	view := overlayCentered(background, "XX\nYY", 12, 4)
	lines := strings.Split(view, "\n")

	if lines[0] != "abcdefghijkl" || lines[3] != "ABCDEFGHIJKL" {
		t.Fatalf("overlay changed an untouched row: %q", view)
	}
	if !strings.HasPrefix(lines[1], "mnopq") || !strings.Contains(lines[1], "XX") || !strings.HasSuffix(lines[1], "tuvwx") {
		t.Fatalf("overlay erased uncovered background: %q", lines[1])
	}
	if !strings.Contains(lines[1], "XX") || !strings.Contains(lines[2], "YY") {
		t.Fatalf("overlay was not centered over the background: %q", view)
	}
}

func TestDashboardMessageAppearsBeforeMemorySummary(t *testing.T) {
	model, backend := newTestModel()
	model.applySnapshot(backend.snapshot)
	model.Message = "Falha ao atualizar: offline"
	view := model.View()
	messagePosition := strings.Index(view, model.Message)
	memoryPosition := strings.Index(view, "USO DA MEMÓRIA")
	if messagePosition < 0 || memoryPosition < 0 || messagePosition > memoryPosition {
		t.Fatal("dashboard message is not visible near the top of the interface")
	}
}

func TestViewHonorsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model, backend := newTestModel()
	model.theme = NewTheme()
	model.applySnapshot(backend.snapshot)
	view := model.View()
	if strings.Contains(view, "\x1b[38;") || strings.Contains(view, "\x1b[48;") {
		t.Fatal("NO_COLOR view contains foreground or background color sequences")
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	confirmation := updated.(Model).View()
	if strings.Contains(confirmation, "\x1b[38;") || strings.Contains(confirmation, "\x1b[48;") {
		t.Fatal("NO_COLOR confirmation contains foreground or background color sequences")
	}
}

func TestThemeModes(t *testing.T) {
	for _, mode := range []string{"auto", "dark", "light", "mono", "cyber"} {
		if _, err := ThemeFor(mode); err != nil {
			t.Fatalf("ThemeFor(%q) error = %v", mode, err)
		}
	}
	if _, err := ThemeFor("unknown"); err == nil {
		t.Fatal("expected invalid theme to fail")
	}
}

func TestLeakAssessmentAndGroupingView(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model, backend := newTestModel()
	model.Width = 120
	model.Height = 30
	model.ShowSplash = false

	// Inject leak assessment on first process
	snapshot := backend.snapshot
	snapshot.Processes[0].Leak = processdomain.LeakAssessment{
		Status:               processdomain.LeakSuspected,
		GrowthBytesPerSecond: 1024,
		Confidence:           0.85,
	}
	snapshot.Processes[0].Category = processdomain.CategoryBrowser
	model.applySnapshot(snapshot)

	view := model.View()
	if !strings.Contains(view, "LEAK GUARD") || !strings.Contains(view, "suspeito") || !strings.Contains(view, "+60.00 KB/min") {
		t.Fatalf("details view omitted leak guard assessment: %s", view)
	}

	// Switch to tree group
	model.groupMode = groupTree
	model.rebuildProcessView()
	treeView := model.View()
	if !strings.Contains(treeView, "└ ") {
		t.Fatal("tree grouping omitted tree branch character '└ '")
	}
}

func TestWideDetailsCardRendersDiscoveredContexts(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model, backend := newTestModel()
	model.Width = 120
	model.Height = 30
	model.ShowSplash = false

	snapshot := backend.snapshot
	snapshot.Processes[0].Contexts = []processdomain.ContextTag{processdomain.ContextDockerCompose}
	model.applySnapshot(snapshot)

	view := model.View()
	if !strings.Contains(view, "CONTEXTO") || !strings.Contains(view, "docker-compose") {
		t.Fatalf("details view omitted discovered context: %s", view)
	}
}

func TestUltrawideGridExpandsUserAndProcessColumns(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model, backend := newTestModel()
	model.Width = 200
	model.Height = 35
	model.ShowSplash = false

	snapshot := backend.snapshot
	snapshot.Processes[0].User = "wesleyximenes"
	snapshot.Processes[0].CommandLine = "chrome --type=renderer --field-trial-handle=0"
	model.applySnapshot(snapshot)

	view := model.View()
	if !strings.Contains(view, "wesleyximenes") {
		t.Fatalf("ultrawide view truncated username: %s", view)
	}
	if !strings.Contains(view, "--type=renderer") {
		t.Fatalf("ultrawide view omitted command line arguments: %s", view)
	}
}

func TestCategoryTabsRenderAndSwitch(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model, backend := newTestModel()
	model.Width = 140
	model.Height = 35
	model.ShowSplash = false
	model.applySnapshot(backend.snapshot)

	view := model.View()
	if !strings.Contains(view, "1:Todos") || !strings.Contains(view, "2:Navegadores") {
		t.Fatalf("view omitted category tabs: %s", view)
	}

	// Press key "2" to filter by Browser
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m := updated.(Model)
	if m.query.Category != processdomain.CategoryBrowser {
		t.Fatalf("query.Category = %q; want browser", m.query.Category)
	}

	// Press tab to cycle
	updated2, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m2 := updated2.(Model)
	if m2.query.Category != processdomain.CategoryContainer {
		t.Fatalf("query.Category after tab = %q; want container", m2.query.Category)
	}
}

func TestContainerEntityRendering(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model, backend := newTestModel()
	model.Width = 140
	model.Height = 35
	model.ShowSplash = false

	snapshot := backend.snapshot
	snapshot.Processes[0].Category = processdomain.CategoryContainer
	snapshot.Processes[0].ContainerName = "sangati_postgres"
	snapshot.Processes[0].ImageName = "postgres:15-alpine"
	snapshot.Processes[0].Command = "postgres"
	snapshot.Processes[0].CommandLine = "docker-entrypoint.sh postgres"
	model.applySnapshot(snapshot)

	view := model.View()
	if !strings.Contains(view, "sangati_postgres") {
		t.Fatalf("table view omitted ContainerName: %s", view)
	}
	if !strings.Contains(view, "IMAGEM") || !strings.Contains(view, "postgres:15-alpine") {
		t.Fatalf("details view omitted ImageName: %s", view)
	}
}





