package daemon

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"sopro/internal/audit"
	processdomain "sopro/internal/process"
)

type mockAuditRecorder struct {
	events []audit.Event
}

func (m *mockAuditRecorder) Record(e audit.Event) error {
	m.events = append(m.events, e)
	return nil
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
