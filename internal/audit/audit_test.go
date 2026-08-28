package audit

import (
	"bytes"
	"encoding/json"
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
