package tui

import (
	"fmt"
	"time"

	"memcleaner/internal/system"

	tea "github.com/charmbracelet/bubbletea"
)

type tickMsg time.Time

type Model struct {
	Manager   system.PlatformManager
	Metrics   system.SystemMetrics
	Processes []system.ProcessInfo
	Cursor    int
	Width     int
	Height    int
	IsRoot    bool
	Message   string
	ScrollPos int // Controla o viewport para rolagem de dados
}

func NewModel(mgr system.PlatformManager) Model {
	metrics, _ := mgr.GetMetrics()
	procs, _ := mgr.GetProcesses(50)
	return Model{
		Manager:   mgr,
		Metrics:   metrics,
		Processes: procs,
		IsRoot:    system.IsRoot(),
	}
}

func (m Model) Init() tea.Cmd {
	return doTick()
}

func doTick() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up":
			if m.Cursor > 0 {
				m.Cursor--
			}
		case "down":
			if m.Cursor < len(m.Processes)-1 {
				m.Cursor++
			}
		case "k":
			if len(m.Processes) > 0 {
				p := m.Processes[m.Cursor]
				err := m.Manager.KillProcess(p.PID)
				if err != nil {
					m.Message = fmt.Sprintf("Error killing process %d: %v", p.PID, err)
				} else {
					m.Message = fmt.Sprintf("SIGKILL sent to %s (PID: %d)", p.Command, p.PID)
				}
			}
		case "c":
			if !m.IsRoot {
				m.Message = "Sudo required to drop caches"
			} else {
				_, err := m.Manager.CleanSystemCache()
				if err != nil {
					m.Message = fmt.Sprintf("Drop cache error: %v", err)
				} else {
					m.Message = "RAM Caches dropped successfully"
				}
			}
		}

	case tickMsg:
		metrics, err1 := m.Manager.GetMetrics()
		procs, err2 := m.Manager.GetProcesses(50)
		if err1 != nil {
			m.Message = fmt.Sprintf("Metrics Error: %v", err1)
		} else if err2 != nil {
			m.Message = fmt.Sprintf("Process Error: %v", err2)
		} else {
			m.Metrics = metrics
			m.Processes = procs
			m.Message = ""
		}
		if m.Cursor >= len(m.Processes) && len(m.Processes) > 0 {
			m.Cursor = len(m.Processes) - 1
		}
		return m, doTick()
	}
	return m, nil
}

func (m Model) View() string {
	s := fmt.Sprintf("MemCleaner (Root: %v) | %s\n", m.IsRoot, m.Message)
	s += fmt.Sprintf("RAM: %d MB Used / %d MB Total | Cache: %d MB\n\n", m.Metrics.UsedRAM/1024/1024, m.Metrics.TotalRAM/1024/1024, m.Metrics.CacheRAM/1024/1024)

	s += fmt.Sprintf("  %-8s %-12s %-6s %-6s %s\n", "PID", "USER", "MEM%", "CPU%", "COMMAND")
	for i, p := range m.Processes {
		cursor := " "
		if m.Cursor == i {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %-8d %-12s %-6.1f%% %-6.1f%% %s\n", cursor, p.PID, p.User, p.MemPct, p.CPUPct, p.Command)
	}
	s += "\n[k] Kill Selected  [c] Drop Caches  [q] Quit\n"
	return s
}
