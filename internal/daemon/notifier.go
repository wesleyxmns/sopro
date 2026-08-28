package daemon

import (
	"fmt"
	"io"
	"strings"
	"time"

	"sopro/internal/audit"
)

type LogNotifier struct {
	writer io.Writer
}

func NewLogNotifier(w io.Writer) *LogNotifier {
	return &LogNotifier{writer: w}
}

func (l *LogNotifier) Notify(d Decision) {
	if l == nil || l.writer == nil {
		return
	}

	ts := d.Timestamp.Format("2006-01-02 15:04:05")
	var status string

	if d.Executed {
		status = "AÇÃO EXECUTADA"
	} else if d.UnderPressure {
		status = "ALERTA PRESSÃO"
	} else {
		status = "NORMAL"
	}

	var pids []string
	for _, id := range d.SuspectedProcesses {
		pids = append(pids, fmt.Sprintf("PID %d", id.PID))
	}

	msg := fmt.Sprintf("[%s] [%s] %s", ts, status, d.Reason)
	if d.RecommendedAction != "" {
		msg += fmt.Sprintf(" · ação recomendada: %s", d.RecommendedAction)
	}
	if len(pids) > 0 {
		msg += fmt.Sprintf(" · processos suspeitos: %s", strings.Join(pids, ", "))
	}
	msg += "\n"

	_, _ = io.WriteString(l.writer, msg)
}

type AuditNotifier struct {
	recorder audit.Recorder
}

func NewAuditNotifier(r audit.Recorder) *AuditNotifier {
	return &AuditNotifier{recorder: r}
}

func (a *AuditNotifier) Notify(d Decision) {
	if a == nil || a.recorder == nil {
		return
	}

	if !d.UnderPressure && !d.Executed && len(d.SuspectedProcesses) == 0 {
		return
	}

	action := "daemon-observe"
	if d.Executed {
		action = "daemon-remedy"
	} else if d.UnderPressure {
		action = "daemon-alert"
	}

	event := audit.Event{
		Action:     action,
		StartedAt:  d.Timestamp,
		FinishedAt: time.Now(),
		Success:    d.Executed || !d.UnderPressure,
		Error:      d.Reason,
	}

	if d.TargetProcess != nil {
		event.PID = d.TargetProcess.PID
		event.ProcessStartedAt = d.TargetProcess.StartedAt
	} else if len(d.SuspectedProcesses) > 0 {
		event.PID = d.SuspectedProcesses[0].PID
		event.ProcessStartedAt = d.SuspectedProcesses[0].StartedAt
	}

	_ = a.recorder.Record(event)
}

type MultiNotifier struct {
	notifiers []Notifier
}

func NewMultiNotifier(notifiers ...Notifier) *MultiNotifier {
	m := &MultiNotifier{}
	for _, n := range notifiers {
		if n != nil {
			m.notifiers = append(m.notifiers, n)
		}
	}
	return m
}

func (m *MultiNotifier) Notify(d Decision) {
	if m == nil {
		return
	}
	for _, n := range m.notifiers {
		n.Notify(d)
	}
}
