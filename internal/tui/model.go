package tui

import (
	"fmt"
	"time"
	"memcleaner/internal/system"
	tea "github.com/charmbracelet/bubbletea"
)

type tickMsg time.Time

type Model struct {
	Metrics   system.SystemMetrics
	Processes []system.ProcessInfo
	Cursor    int
	IsRoot    bool
	Message   string
}

func NewModel() Model {
	m := Model{
		IsRoot: system.IsRoot(),
	}
	metrics, err := system.GetSystemMetrics()
	if err != nil {
		m.Message = fmt.Sprintf("Error getting metrics: %v", err)
	} else {
		m.Metrics = metrics
	}

	procs, err := system.GetTopProcesses(50)
	if err != nil {
		msg := fmt.Sprintf("Error getting processes: %v", err)
		if m.Message == "" {
			m.Message = msg
		} else {
			m.Message += " | " + msg
		}
	} else {
		m.Processes = procs
	}
	return m
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
				pid := m.Processes[m.Cursor].PID
				err := system.KillProcess(pid)
				if err != nil {
					m.Message = fmt.Sprintf("Error killing PID %d: %v", pid, err)
				} else {
					m.Message = fmt.Sprintf("SIGKILL sent to PID %d", pid)
				}
			}
		case "c":
			if !m.IsRoot {
				m.Message = "Sudo required to drop caches"
			} else {
				err := system.DropCaches()
				if err != nil {
					m.Message = fmt.Sprintf("Drop cache error: %v", err)
				} else {
					m.Message = "RAM Caches dropped successfully"
				}
			}
		}
	case tickMsg:
		metrics, err := system.GetSystemMetrics()
		if err != nil {
			m.Message = fmt.Sprintf("Error getting metrics: %v", err)
		} else {
			m.Metrics = metrics
		}

		procs, err := system.GetTopProcesses(50)
		if err != nil {
			procMsg := fmt.Sprintf("Error getting processes: %v", err)
			if m.Message == "" {
				m.Message = procMsg
			} else {
				m.Message += " | " + procMsg
			}
		} else {
			m.Processes = procs
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

	s += "PID\tUSER\tMEM%%\tCPU%%\tCOMMAND\n"
	for i, p := range m.Processes {
		cursor := " "
		if m.Cursor == i {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %d\t%s\t%.1f%%\t%.1f%%\t%s\n", cursor, p.PID, p.User, p.MemPct, p.CPUPct, p.Command)
	}
	s += "\n[k] Kill Selected  [c] Drop Caches  [q] Quit\n"
	return s
}
