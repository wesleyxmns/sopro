package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wesleyxmns/sopro/internal/audit"
	processdomain "github.com/wesleyxmns/sopro/internal/process"
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

	if d.Failed {
		status = "FALHA"
	} else if d.Executed {
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

// JSONNotifier emits one JSON decision per line for log scraping.
type JSONNotifier struct {
	writer io.Writer
}

func NewJSONNotifier(w io.Writer) *JSONNotifier {
	return &JSONNotifier{writer: w}
}

func (n *JSONNotifier) Notify(d Decision) {
	if n == nil || n.writer == nil {
		return
	}
	payload, err := json.Marshal(newDecisionPayload(d))
	if err != nil {
		return
	}
	_, _ = io.WriteString(n.writer, string(payload)+"\n")
}

// WebhookNotifier POSTs decisions to an HTTP endpoint. To avoid spamming, it
// only fires on transitions into an interesting state and on every executed
// remedy; delivery failures are silently dropped like other notifiers.
type WebhookNotifier struct {
	mu       sync.Mutex
	url      string
	client   *http.Client
	alerting bool
}

func NewWebhookNotifier(url string) *WebhookNotifier {
	return &WebhookNotifier{url: url, client: &http.Client{Timeout: 5 * time.Second}}
}

func (n *WebhookNotifier) Notify(d Decision) {
	if n == nil || n.url == "" {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	interesting := d.UnderPressure || d.Executed || len(d.SuspectedProcesses) > 0
	if !interesting {
		n.alerting = false
		return
	}
	if n.alerting && !d.Executed {
		return
	}
	n.alerting = true
	n.post(d)
}

func (n *WebhookNotifier) post(d Decision) {
	payload, err := json.Marshal(newDecisionPayload(d))
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

type decisionPayload struct {
	Timestamp          time.Time               `json:"timestamp"`
	UnderPressure      bool                    `json:"under_pressure"`
	PressureDuration   string                  `json:"pressure_duration"`
	RecommendedAction  string                  `json:"recommended_action,omitempty"`
	Executed           bool                    `json:"executed"`
	Failed             bool                    `json:"failed"`
	Reason             string                  `json:"reason"`
	SuspectedProcesses []processdomainIdentity `json:"suspected_processes,omitempty"`
}

func newDecisionPayload(d Decision) decisionPayload {
	return decisionPayload{
		Timestamp:          d.Timestamp,
		UnderPressure:      d.UnderPressure,
		PressureDuration:   d.PressureDuration.String(),
		RecommendedAction:  d.RecommendedAction,
		Executed:           d.Executed,
		Failed:             d.Failed,
		Reason:             d.Reason,
		SuspectedProcesses: identities(d.SuspectedProcesses),
	}
}

type processdomainIdentity struct {
	PID       int32 `json:"pid"`
	StartedAt int64 `json:"started_at"`
}

func identities(ids []processdomain.Identity) []processdomainIdentity {
	converted := make([]processdomainIdentity, 0, len(ids))
	for _, id := range ids {
		converted = append(converted, processdomainIdentity{PID: id.PID, StartedAt: id.StartedAt})
	}
	return converted
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
	if d.Executed || d.Failed {
		action = "daemon-remedy"
	} else if d.UnderPressure {
		action = "daemon-alert"
	}

	event := audit.Event{
		Action:     action,
		StartedAt:  d.Timestamp,
		FinishedAt: time.Now(),
		Success:    !d.Failed,
	}
	if d.Failed {
		event.Error = d.Reason
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
