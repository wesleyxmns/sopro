package audit

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestJSONLRecorderWritesOneEventPerLine(t *testing.T) {
	var output bytes.Buffer
	recorder := NewJSONLRecorder(&output)
	event := Event{Action: "kill", PID: 42, StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0), Success: true}
	if err := recorder.Record(event); err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(output.String(), "\n"); lines != 1 {
		t.Fatalf("audit lines = %d; want 1", lines)
	}
	var decoded Event
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Action != event.Action || decoded.PID != event.PID || !decoded.Success {
		t.Fatalf("decoded event = %+v", decoded)
	}
}

func TestOpenRejectsEmptyPath(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatal("expected empty audit path to fail")
	}
}

func TestRotationBoundsLogFiles(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/actions.jsonl"
	recorder, err := OpenWithRotation(path, 300, 2)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		event := Event{Action: "kill", PID: int32(i), StartedAt: time.Unix(int64(i), 0), Success: true}
		if err := recorder.Record(event); err != nil {
			t.Fatal(err)
		}
	}
	if err := recorder.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > 3 {
		t.Fatalf("log files = %d; want at most 3 (current + 2 backups)", len(entries))
	}
	events, err := ReadLast(path, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 || events[len(events)-1].PID != 9 {
		t.Fatalf("latest event missing after rotation: %+v", events)
	}
}

func TestReadLastSpansBackupsAndSkipsMalformed(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/actions.jsonl"
	write := func(name, lines string) {
		t.Helper()
		if err := os.WriteFile(dir+"/"+name, []byte(lines), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("actions.jsonl.1", "{\"action\":\"old\",\"success\":true}\nnot-json\n")
	write("actions.jsonl", "{\"action\":\"new\",\"success\":false,\"error\":\"boom\"}\n")

	events, err := ReadLast(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Action != "old" || events[1].Action != "new" {
		t.Fatalf("events = %+v; want old then new, malformed skipped", events)
	}
	if events[1].Error != "boom" {
		t.Fatalf("new event error = %q; want boom", events[1].Error)
	}

	latest, err := ReadLast(path, 1)
	if err != nil || len(latest) != 1 || latest[0].Action != "new" {
		t.Fatalf("latest = %+v, err = %v", latest, err)
	}

	empty, err := ReadLast(dir+"/missing.jsonl", 10)
	if err != nil || len(empty) != 0 {
		t.Fatalf("missing log = %+v, err = %v; want empty", empty, err)
	}
}
