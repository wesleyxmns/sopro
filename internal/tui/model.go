package tui

import (
	"errors"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/wesleyxmns/sopro/internal/app"
	"github.com/wesleyxmns/sopro/internal/control"
	"github.com/wesleyxmns/sopro/internal/memory"
	processdomain "github.com/wesleyxmns/sopro/internal/process"
	"github.com/wesleyxmns/sopro/internal/updater"
	"github.com/wesleyxmns/sopro/internal/version"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	service         *app.Service
	Snapshot        app.Snapshot
	Capabilities    app.Capabilities
	Cursor          int
	Width           int
	Height          int
	Message         string
	Loading         bool
	Acting          bool
	ShowSplash      bool
	Pending         *control.Request
	ActiveAction    *control.Request
	UpdateAvailable *updater.ReleaseInfo
	PendingUpdate   *updater.ReleaseInfo
	viewport        viewport.Model
	theme           Theme
	memoryHistory   []memory.Snapshot
	allProcesses    []processdomain.Info
	query           processdomain.Query
	groupMode       processGroupMode
	processDepth    map[processdomain.Identity]int
	Searching       bool
}

type processGroupMode int

const (
	groupFlat processGroupMode = iota
	groupCategory
	groupTree
)

type Option func(*Model)

func WithTheme(theme Theme) Option {
	return func(model *Model) {
		model.theme = theme
	}
}

func NewModel(service *app.Service, options ...Option) Model {
	model := Model{
		service:      service,
		Capabilities: service.Capabilities(),
		Loading:      true,
		ShowSplash:   true,
		viewport:     viewport.New(1, 1),
		theme:        NewTheme(),
		query:        processdomain.Query{},
		processDepth: make(map[processdomain.Identity]int),
	}
	for _, option := range options {
		option(&model)
	}
	return model
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadSnapshotCmd(m.service),
		scheduleTick(),
		finishSplashCmd(),
		checkUpdateCmd(version.Version),
	)
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.Width, m.Height = msg.Width, msg.Height
		m.syncViewport()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tickMsg:
		if m.Loading {
			return m, scheduleTick()
		}
		m.Loading = true
		return m, tea.Batch(loadSnapshotCmd(m.service), scheduleTick())

	case splashFinishedMsg:
		m.ShowSplash = false
		return m, nil

	case snapshotLoadedMsg:
		m.Loading = false
		if msg.err != nil {
			m.Message = "Falha ao atualizar: " + msg.err.Error()
			return m, nil
		}
		m.applySnapshot(msg.snapshot)
		m.Message = ""
		return m, nil

	case actionFinishedMsg:
		m.Acting = false
		m.ActiveAction = nil
		if msg.err != nil {
			var actionErr *app.ActionError
			if errors.As(msg.err, &actionErr) && actionErr.Operation == nil {
				m.Message = "Ação concluída, mas a auditoria falhou: " + actionErr.Audit.Error()
				m.Loading = true
				return m, loadSnapshotCmd(m.service)
			}
			m.Message = "Ação falhou: " + msg.err.Error()
			return m, nil
		}
		if msg.result.Action == control.ActionClean {
			m.Message = "Cache limpo: " + memory.FormatBytes(msg.result.Reclaimed)
		} else {
			m.Message = fmt.Sprintf("Ação %s concluída no PID %d", msg.result.Action, msg.result.Process.PID)
		}
		m.Loading = true
		return m, loadSnapshotCmd(m.service)

	case updateCheckedMsg:
		if msg.err == nil && msg.isNew && msg.release != nil {
			m.UpdateAvailable = msg.release
		}
		return m, nil

	case updateAppliedMsg:
		m.Acting = false
		m.ActiveAction = nil
		if msg.err != nil {
			m.Message = "Falha na atualização: " + msg.err.Error()
			return m, nil
		}
		m.UpdateAvailable = nil
		m.PendingUpdate = nil
		m.Message = fmt.Sprintf("✔ Sopro atualizado para %s! Reinicie para carregar.", msg.release.TagName)
		return m, nil
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if m.ShowSplash {
		if key == "q" || key == "ctrl+c" {
			return m, tea.Quit
		}
		m.ShowSplash = false
		return m, nil
	}
	if m.Pending != nil {
		switch key {
		case "enter", "y":
			request := *m.Pending
			m.Pending = nil
			m.Acting = true
			m.ActiveAction = &request
			m.Message = "Executando " + string(request.Action) + "…"
			return m, executeActionCmd(m.service, request)
		case "esc", "n":
			m.Pending = nil
			m.Message = "Ação cancelada"
		}
		return m, nil
	}
	if m.PendingUpdate != nil {
		switch key {
		case "enter", "y":
			rel := m.PendingUpdate
			m.PendingUpdate = nil
			m.Acting = true
			m.ActiveAction = &control.Request{Action: "sopro.update"}
			m.Message = "Baixando e instalando atualização " + rel.TagName + "…"
			return m, applyUpdateCmd(rel)
		case "esc", "n":
			m.PendingUpdate = nil
			m.Message = "Atualização cancelada"
		}
		return m, nil
	}
	if m.Acting {
		if key == "q" || key == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}
	if m.Searching {
		return m.handleSearchKey(msg)
	}

	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up":
		m.moveCursor(-1)
	case "down":
		m.moveCursor(1)
	case "x":
		m.prepareProcessAction(control.ActionTerminate)
	case "k", "K":
		m.prepareProcessAction(control.ActionKill)
	case "p":
		if selected, ok := m.selectedProcess(); ok {
			action := control.ActionPause
			if selected.State == processdomain.StatePaused {
				action = control.ActionResume
			}
			m.prepareProcessAction(action)
		}
	case "c":
		if !m.Capabilities.CanCleanCache {
			m.Message = "Limpeza de cache indisponível sem privilégios"
			break
		}
		m.Pending = &control.Request{Action: control.ActionClean}
		m.Message = "Confirmar limpeza manual do cache?"
	case "d":
		if selected, ok := m.selectedProcess(); ok && (selected.Category == processdomain.CategoryContainer || selected.ContainerID != "") {
			name := selected.ContainerName
			if name == "" {
				name = fmt.Sprintf("PID %d", selected.PID)
			}
			if selected.State == processdomain.StateStopped {
				m.Pending = &control.Request{
					Action:        control.ActionDockerStart,
					Process:       selected.Identity,
					ContainerName: selected.ContainerName,
					ContainerID:   selected.ContainerID,
				}
				m.Message = fmt.Sprintf("Confirmar início do container '%s'?", name)
			} else {
				m.Pending = &control.Request{
					Action:        control.ActionDockerStop,
					Process:       selected.Identity,
					ContainerName: selected.ContainerName,
					ContainerID:   selected.ContainerID,
				}
				m.Message = fmt.Sprintf("Confirmar parada do container '%s'?", name)
			}
		}
	case "r":
		if selected, ok := m.selectedProcess(); ok && (selected.Category == processdomain.CategoryContainer || selected.ContainerID != "") {
			name := selected.ContainerName
			if name == "" {
				name = fmt.Sprintf("PID %d", selected.PID)
			}
			m.Pending = &control.Request{
				Action:        control.ActionDockerRestart,
				Process:       selected.Identity,
				ContainerName: selected.ContainerName,
				ContainerID:   selected.ContainerID,
			}
			m.Message = fmt.Sprintf("Confirmar reinício do container '%s'?", name)
		}
	case "z":
		if selected, ok := m.selectedProcess(); ok && (selected.Category == processdomain.CategoryContainer || selected.ContainerID != "") {
			name := selected.ContainerName
			if name == "" {
				name = fmt.Sprintf("PID %d", selected.PID)
			}
			m.Pending = &control.Request{
				Action:        control.ActionDockerPause,
				Process:       selected.Identity,
				ContainerName: selected.ContainerName,
				ContainerID:   selected.ContainerID,
			}
			m.Message = fmt.Sprintf("Confirmar pausa do container '%s'?", name)
		}
	case "b":
		if selected, ok := m.selectedProcess(); ok && (selected.Category == processdomain.CategoryBrowser || hasContextTag(selected.Contexts, processdomain.ContextBrowserDebug)) {
			m.Pending = &control.Request{Action: control.ActionCDPCloseBlank, Process: selected.Identity}
			m.Message = fmt.Sprintf("Confirmar fechamento de abas vazias via CDP (PID %d)?", selected.PID)
		}
	case "u":
		if selected, ok := m.selectedProcess(); ok && (selected.Category == processdomain.CategoryBrowser || hasContextTag(selected.Contexts, processdomain.ContextBrowserDebug)) {
			m.Pending = &control.Request{Action: control.ActionCDPDiscardInactive, Process: selected.Identity}
			m.Message = fmt.Sprintf("Confirmar suspensão de abas inativas via CDP (PID %d)?", selected.PID)
		}
	case "j":
		if selected, ok := m.selectedProcess(); ok && (selected.Category == processdomain.CategoryJVM || hasContextTag(selected.Contexts, processdomain.ContextTag("jvm-runtime"))) {
			m.Pending = &control.Request{Action: control.ActionJVMRunGC, Process: selected.Identity}
			m.Message = fmt.Sprintf("Confirmar Garbage Collection na JVM (PID %d)?", selected.PID)
		}
	case "/":
		m.Searching = true
		m.Message = "Busca fuzzy: digite para filtrar · enter conclui · esc limpa"
	case "f", "tab":
		m.cycleCategoryFilter()
	case "shift+tab":
		m.cycleCategoryFilterReverse()
	case "1":
		m.setCategoryFilter("")
	case "2":
		m.setCategoryFilter(processdomain.CategoryBrowser)
	case "3":
		m.setCategoryFilter(processdomain.CategoryContainer)
	case "4":
		m.setCategoryFilter(processdomain.CategoryDevelopment)
	case "5":
		m.setCategoryFilter(processdomain.CategoryDatabase)
	case "6":
		m.setCategoryFilter(processdomain.CategoryJVM)
	case "7":
		m.setCategoryFilter(processdomain.CategorySystem)
	case "8":
		m.setCategoryFilter(processdomain.CategoryOther)
	case "s":
		if selected, ok := m.selectedProcess(); ok && (selected.Category == processdomain.CategoryContainer || selected.ContainerID != "") && selected.State == processdomain.StateStopped {
			name := selected.ContainerName
			if name == "" {
				name = selected.Command
			}
			m.Pending = &control.Request{
				Action:        control.ActionDockerStart,
				Process:       selected.Identity,
				ContainerName: selected.ContainerName,
				ContainerID:   selected.ContainerID,
			}
			m.Message = fmt.Sprintf("Confirmar início do container '%s'?", name)
			break
		}
		m.cycleSort()
	case "g":
		m.groupMode = (m.groupMode + 1) % 3
		m.rebuildProcessView()
	case "U":
		if m.UpdateAvailable != nil {
			m.PendingUpdate = m.UpdateAvailable
			m.Message = fmt.Sprintf("Confirmar atualização do Sopro para %s?", m.UpdateAvailable.TagName)
		}
	case "esc":
		m.Message = ""
	}
	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.Searching = false
		m.Message = ""
	case tea.KeyEsc:
		m.Searching = false
		m.query.Search = ""
		m.Message = "Busca limpa"
		m.rebuildProcessView()
	case tea.KeyBackspace, tea.KeyDelete:
		if m.query.Search != "" {
			_, size := utf8.DecodeLastRuneInString(m.query.Search)
			m.query.Search = m.query.Search[:len(m.query.Search)-size]
			m.rebuildProcessView()
		}
	case tea.KeyRunes:
		m.query.Search += string(msg.Runes)
		m.rebuildProcessView()
	}
	return m, nil
}

func (m *Model) setCategoryFilter(category processdomain.Category) {
	m.query.Category = category
	m.rebuildProcessView()
}

func (m *Model) cycleCategoryFilter() {
	categories := []processdomain.Category{
		"", processdomain.CategoryBrowser, processdomain.CategoryContainer,
		processdomain.CategoryDevelopment, processdomain.CategoryDatabase,
		processdomain.CategoryJVM, processdomain.CategorySystem, processdomain.CategoryOther,
	}
	index := 0
	for candidate := range categories {
		if categories[candidate] == m.query.Category {
			index = candidate
			break
		}
	}
	m.query.Category = categories[(index+1)%len(categories)]
	m.rebuildProcessView()
}

func (m *Model) cycleCategoryFilterReverse() {
	categories := []processdomain.Category{
		"", processdomain.CategoryBrowser, processdomain.CategoryContainer,
		processdomain.CategoryDevelopment, processdomain.CategoryDatabase,
		processdomain.CategoryJVM, processdomain.CategorySystem, processdomain.CategoryOther,
	}
	index := 0
	for candidate := range categories {
		if categories[candidate] == m.query.Category {
			index = candidate
			break
		}
	}
	newIndex := (index - 1 + len(categories)) % len(categories)
	m.query.Category = categories[newIndex]
	m.rebuildProcessView()
}

func (m *Model) cycleSort() {
	switch m.query.Sort {
	case "":
		m.query.Sort = processdomain.SortCPU
	case processdomain.SortCPU:
		m.query.Sort = processdomain.SortCommand
	case processdomain.SortCommand:
		m.query.Sort = processdomain.SortMemory
	default:
		m.query.Sort = ""
	}
	m.rebuildProcessView()
}

func (m *Model) prepareProcessAction(action control.Action) {
	selected, ok := m.selectedProcess()
	if !ok {
		return
	}
	if !m.supports(action) {
		m.Message = "Ação indisponível nesta plataforma"
		return
	}
	m.Pending = &control.Request{Action: action, Process: selected.Identity}
	m.Message = fmt.Sprintf("Confirmar %s em %s (PID %d, risco %s)?", action, selected.Command, selected.PID, selected.Risk)
}

func (m Model) supports(action control.Action) bool {
	switch action {
	case control.ActionTerminate:
		return m.Capabilities.CanTerminate
	case control.ActionKill:
		return m.Capabilities.CanKill
	case control.ActionPause:
		return m.Capabilities.CanPause
	case control.ActionResume:
		return m.Capabilities.CanResume
	default:
		return false
	}
}

func (m *Model) moveCursor(delta int) {
	m.Cursor += delta
	if m.Cursor < 0 {
		m.Cursor = 0
	}
	if last := len(m.Snapshot.Processes) - 1; m.Cursor > last {
		m.Cursor = max(last, 0)
	}
	m.syncViewport()
}

func (m Model) selectedProcess() (processdomain.Info, bool) {
	if m.Cursor < 0 || m.Cursor >= len(m.Snapshot.Processes) {
		return processdomain.Info{}, false
	}
	return m.Snapshot.Processes[m.Cursor], true
}

func (m *Model) applySnapshot(snapshot app.Snapshot) {
	selected, hadSelection := m.selectedProcess()
	m.Snapshot = snapshot
	m.allProcesses = append([]processdomain.Info(nil), snapshot.Processes...)
	m.memoryHistory = append(m.memoryHistory, snapshot.Memory)
	if len(m.memoryHistory) > 30 {
		m.memoryHistory = append([]memory.Snapshot(nil), m.memoryHistory[len(m.memoryHistory)-30:]...)
	}
	m.rebuildProcessViewWithSelection(selected, hadSelection)
}

func (m *Model) rebuildProcessView() {
	selected, hadSelection := m.selectedProcess()
	m.rebuildProcessViewWithSelection(selected, hadSelection)
}

func (m *Model) rebuildProcessViewWithSelection(selected processdomain.Info, hadSelection bool) {
	processes := m.query.Apply(m.allProcesses)
	m.processDepth = make(map[processdomain.Identity]int, len(processes))
	switch m.groupMode {
	case groupCategory:
		sort.SliceStable(processes, func(left, right int) bool {
			return categoryRank(processes[left].Category) < categoryRank(processes[right].Category)
		})
	case groupTree:
		entries := processdomain.BuildTree(processes)
		processes = processes[:0]
		for _, entry := range entries {
			processes = append(processes, entry.Info)
			m.processDepth[entry.Identity] = entry.Depth
		}
	}
	m.Snapshot.Processes = processes
	if hadSelection {
		for index, candidate := range processes {
			if candidate.Identity == selected.Identity {
				m.Cursor = index
				m.syncViewport()
				return
			}
		}
	}
	if m.Cursor >= len(processes) {
		m.Cursor = max(len(processes)-1, 0)
	}
	m.syncViewport()
}

func categoryRank(category processdomain.Category) int {
	order := []processdomain.Category{
		processdomain.CategorySystem, processdomain.CategoryContainer,
		processdomain.CategoryBrowser, processdomain.CategoryDatabase,
		processdomain.CategoryDevelopment, processdomain.CategoryJVM,
		processdomain.CategoryOther,
	}
	for index, candidate := range order {
		if candidate == category {
			return index
		}
	}
	return len(order)
}

func (m *Model) syncViewport() {
	layout := calculateLayout(m.Width, m.Height)
	m.viewport.Width = layout.listWidth
	m.viewport.Height = layout.viewportHeight
	m.viewport.SetContent(m.renderProcessRows(layout.listWidth, layout.mode))
	if m.Cursor < m.viewport.YOffset {
		m.viewport.SetYOffset(m.Cursor)
	}
	if m.Cursor >= m.viewport.YOffset+m.viewport.Height {
		m.viewport.SetYOffset(m.Cursor - m.viewport.Height + 1)
	}
}
