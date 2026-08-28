package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Event struct {
	Action           string        `json:"action"`
	PID              int32         `json:"pid,omitempty"`
	ProcessStartedAt int64         `json:"process_started_at,omitempty"`
	StartedAt        time.Time     `json:"started_at"`
	FinishedAt       time.Time     `json:"finished_at"`
	Duration         time.Duration `json:"duration_ns"`
	Success          bool          `json:"success"`
	Error            string        `json:"error,omitempty"`
	Escalated        bool          `json:"escalated,omitempty"`
	ReclaimedBytes   uint64        `json:"reclaimed_bytes,omitempty"`
}

type Recorder interface {
	Record(Event) error
}

type JSONLRecorder struct {
	mu     sync.Mutex
	writer io.Writer
	closer io.Closer
}

func NewJSONLRecorder(writer io.Writer) *JSONLRecorder {
	return &JSONLRecorder{writer: writer}
}

func Open(path string) (*JSONLRecorder, error) {
	if path == "" {
		return nil, fmt.Errorf("audit path must not be empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create audit directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open audit log: %w", err)
	}
	return &JSONLRecorder{writer: file, closer: file}, nil
}

func DefaultPath() string {
	directory, err := os.UserConfigDir()
	if err != nil || directory == "" {
		return "sopro-actions.jsonl"
	}
	return filepath.Join(directory, "sopro", "actions.jsonl")
}

func (r *JSONLRecorder) Record(event Event) error {
	if r == nil || r.writer == nil {
		return fmt.Errorf("audit recorder is not initialized")
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode audit event: %w", err)
	}
	payload = append(payload, '\n')
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.writer.Write(payload); err != nil {
		return fmt.Errorf("write audit event: %w", err)
	}
	return nil
}

func (r *JSONLRecorder) Close() error {
	if r == nil || r.closer == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closer.Close()
}
