package daemon

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wesleyxmns/sopro/internal/audit"
	processdomain "github.com/wesleyxmns/sopro/internal/process"
)

type mockAuditRecorder struct {
	events []audit.Event
}

func (m *mockAuditRecorder) Record(e audit.Event) error {
	m.events = append(m.events, e)
	return nil
}

func TestJSONNotifierEmitsOneDecisionPerLine(t *testing.T) {
	buf := &bytes.Buffer{}
	notifier := NewJSONNotifier(buf)

	decision := Decision{
		Timestamp:          time.Date(2026, 8, 28, 16, 30, 0, 0, time.UTC),
		UnderPressure:      true,
		PressureDuration:   15 * time.Second,
		RecommendedAction:  "clean-cache",
		Reason:             "modo observação: limpeza recomendada",
		SuspectedProcesses: []processdomain.Identity{{PID: 101}},
	}
	notifier.Notify(decision)
	output := buf.String()
	for _, expected := range []string{
		`"under_pressure":true`, `"pressure_duration":"15s"`,
		`"recommended_action":"clean-cache"`, `"pid":101`,
		`"executed":false`, `"failed":false`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("JSON output omitted %s: %s", expected, output)
		}
	}
	if lines := strings.Count(output, "\n"); lines != 1 {
		t.Fatalf("JSON lines = %d; want 1", lines)
	}
}

func TestWebhookNotifierFiresOnTransitionsAndRemedies(t *testing.T) {
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(payload))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notifier := NewWebhookNotifier(server.URL)
	normal := Decision{Timestamp: time.Now(), Reason: "pressão normal"}
	pressure := Decision{Timestamp: time.Now(), UnderPressure: true, Reason: "pressão elevada"}
	remedy := Decision{Timestamp: time.Now(), UnderPressure: true, Executed: true, Reason: "limpeza executada"}

	notifier.Notify(normal)
	notifier.Notify(pressure)
	notifier.Notify(pressure)
	notifier.Notify(remedy)
	notifier.Notify(normal)
	notifier.Notify(pressure)

	if len(bodies) != 3 {
		t.Fatalf("webhook calls = %d; want 3 (transition, remedy, re-alert)", len(bodies))
	}
	if !strings.Contains(bodies[0], `"under_pressure":true`) {
		t.Fatalf("first payload = %s; want pressure decision", bodies[0])
	}
	if !strings.Contains(bodies[1], `"executed":true`) {
		t.Fatalf("second payload = %s; want executed remedy", bodies[1])
	}

	quiet := NewWebhookNotifier("")
	quiet.Notify(pressure)

	var nilNotifier *WebhookNotifier
	nilNotifier.Notify(pressure)
}

func TestLogNotifierFormatsDecisions(t *testing.T) {
	buf := &bytes.Buffer{}
	notifier := NewLogNotifier(buf)

	decision := Decision{
		Timestamp:          time.Date(2026, 8, 28, 16, 30, 0, 0, time.UTC),
		UnderPressure:      true,
		PressureDuration:   15 * time.Second,
		RecommendedAction:  "clean-cache",
		Reason:             "modo observação: limpeza recomendada",
		SuspectedProcesses: []processdomain.Identity{{PID: 101}, {PID: 202}},
	}

	notifier.Notify(decision)
	output := buf.String()

	if !strings.Contains(output, "2026-08-28 16:30:00") {
		t.Fatalf("missing timestamp in output: %s", output)
	}
	if !strings.Contains(output, "ALERTA PRESSÃO") {
		t.Fatalf("missing status in output: %s", output)
	}
	if !strings.Contains(output, "ação recomendada: clean-cache") {
		t.Fatalf("missing action in output: %s", output)
	}
	if !strings.Contains(output, "PID 101, PID 202") {
		t.Fatalf("missing suspected pids in output: %s", output)
	}
}

func TestAuditNotifierRecordsEvents(t *testing.T) {
	recorder := &mockAuditRecorder{}
	notifier := NewAuditNotifier(recorder)

	// Normal decision with no leaks is ignored by audit notifier
	notifier.Notify(Decision{UnderPressure: false, Reason: "pressão normal"})
	if len(recorder.events) != 0 {
		t.Fatalf("expected 0 events for normal decision, got %d", len(recorder.events))
	}

	// Alert decision with leak
	alertDecision := Decision{
		Timestamp:          time.Now(),
		UnderPressure:      true,
		Reason:             "pressão PSI alta",
		SuspectedProcesses: []processdomain.Identity{{PID: 404, StartedAt: 9999}},
	}
	notifier.Notify(alertDecision)
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 event for alert, got %d", len(recorder.events))
	}
	if recorder.events[0].Action != "daemon-alert" || recorder.events[0].PID != 404 {
		t.Fatalf("unexpected event: %+v", recorder.events[0])
	}

	// Remedy executed decision
	remedyDecision := Decision{
		Timestamp:         time.Now(),
		UnderPressure:     true,
		Executed:          true,
		RecommendedAction: "clean-cache",
		Reason:            "limpeza executada",
	}
	notifier.Notify(remedyDecision)
	if len(recorder.events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(recorder.events))
	}
	if recorder.events[1].Action != "daemon-remedy" || !recorder.events[1].Success {
		t.Fatalf("unexpected event: %+v", recorder.events[1])
	}
}

func TestAuditNotifierTreatsObservationAsSuccess(t *testing.T) {
	recorder := &mockAuditRecorder{}
	notifier := NewAuditNotifier(recorder)

	notifier.Notify(Decision{
		Timestamp:         time.Now(),
		UnderPressure:     true,
		RecommendedAction: "clean-cache",
		Reason:            "modo observação: limpeza de cache recomendada mas não executada",
	})
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 event for observe-mode alert, got %d", len(recorder.events))
	}
	event := recorder.events[0]
	if event.Action != "daemon-alert" {
		t.Fatalf("action = %q; want daemon-alert", event.Action)
	}
	if !event.Success {
		t.Fatalf("observe-mode alert recorded as failure: %+v", event)
	}
	if event.Error != "" {
		t.Fatalf("observe-mode alert recorded error %q; want empty", event.Error)
	}
}

func TestAuditNotifierRecordsFailedRemedy(t *testing.T) {
	recorder := &mockAuditRecorder{}
	notifier := NewAuditNotifier(recorder)

	notifier.Notify(Decision{
		Timestamp:         time.Now(),
		UnderPressure:     true,
		RecommendedAction: "clean-cache",
		Failed:            true,
		Reason:            "execução de limpeza de cache falhou: permissão negada",
	})
	if len(recorder.events) != 1 {
		t.Fatalf("expected 1 event for failed remedy, got %d", len(recorder.events))
	}
	event := recorder.events[0]
	if event.Action != "daemon-remedy" {
		t.Fatalf("action = %q; want daemon-remedy", event.Action)
	}
	if event.Success {
		t.Fatalf("failed remedy recorded as success: %+v", event)
	}
	if !strings.Contains(event.Error, "permissão negada") {
		t.Fatalf("error = %q; want failure reason", event.Error)
	}
}

func TestLogNotifierMarksFailedRemedy(t *testing.T) {
	buf := &bytes.Buffer{}
	notifier := NewLogNotifier(buf)

	notifier.Notify(Decision{
		Timestamp:     time.Date(2026, 8, 28, 16, 30, 0, 0, time.UTC),
		UnderPressure: true,
		Failed:        true,
		Reason:        "execução de limpeza de cache falhou: permissão negada",
	})
	if output := buf.String(); !strings.Contains(output, "FALHA") {
		t.Fatalf("missing FALHA status in output: %s", output)
	}
}

func TestMultiNotifierDispatchesToAll(t *testing.T) {
	buf := &bytes.Buffer{}
	logN := NewLogNotifier(buf)
	rec := &mockAuditRecorder{}
	auditN := NewAuditNotifier(rec)

	multi := NewMultiNotifier(logN, auditN)

	decision := Decision{
		Timestamp:     time.Now(),
		UnderPressure: true,
		Reason:        "alerta teste",
	}

	multi.Notify(decision)

	if buf.Len() == 0 {
		t.Fatal("log notifier did not receive notification")
	}
	if len(rec.events) != 1 {
		t.Fatal("audit notifier did not receive notification")
	}
}
