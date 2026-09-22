package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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
	mu         sync.Mutex
	writer     io.Writer
	closer     io.Closer
	path       string
	maxBytes   int64
	maxBackups int
}

// Rotation defaults keep the audit trail bounded: 5 MB per file and 3 backups.
const (
	DefaultMaxBytes   int64 = 5 * 1024 * 1024
	DefaultMaxBackups       = 3
)

func NewJSONLRecorder(writer io.Writer) *JSONLRecorder {
	return &JSONLRecorder{writer: writer}
}

func Open(path string) (*JSONLRecorder, error) {
	return OpenWithRotation(path, DefaultMaxBytes, DefaultMaxBackups)
}

func OpenWithRotation(path string, maxBytes int64, maxBackups int) (*JSONLRecorder, error) {
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
	return &JSONLRecorder{writer: file, closer: file, path: path, maxBytes: maxBytes, maxBackups: maxBackups}, nil
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
	r.rotateIfNeeded(int64(len(payload)))
	if _, err := r.writer.Write(payload); err != nil {
		return fmt.Errorf("write audit event: %w", err)
	}
	return nil
}

// rotateIfNeeded moves a full log aside as path.1 (shifting older backups and
// dropping beyond maxBackups). Rotation is best-effort: failures fall back to
// appending, and write errors still surface from Record.
func (r *JSONLRecorder) rotateIfNeeded(incoming int64) {
	if r.path == "" || r.maxBytes <= 0 {
		return
	}
	info, err := os.Stat(r.path)
	if err != nil || info.Size()+incoming <= r.maxBytes {
		return
	}
	if closer, ok := r.closer.(*os.File); ok && closer != nil {
		_ = closer.Close()
	}
	if r.maxBackups <= 0 {
		_ = os.Remove(r.path)
	} else {
		for index := r.maxBackups - 1; index >= 1; index-- {
			_ = os.Rename(backupPath(r.path, index), backupPath(r.path, index+1))
		}
		_ = os.Rename(r.path, backupPath(r.path, 1))
	}
	if file, err := os.OpenFile(r.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		r.writer, r.closer = file, file
	}
}

func backupPath(path string, index int) string {
	return fmt.Sprintf("%s.%d", path, index)
}

func (r *JSONLRecorder) Close() error {
	if r == nil || r.closer == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closer.Close()
}

// ReadLast returns up to the n most recent events, oldest first, spanning the
// current log and any rotated backups. Malformed lines are skipped and a
// missing log reads as empty.
func ReadLast(path string, n int) ([]Event, error) {
	if n <= 0 {
		return nil, nil
	}
	var events []Event
	for _, file := range logFiles(path) {
		payload, err := os.ReadFile(file)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read audit log: %w", err)
		}
		for _, line := range strings.Split(string(payload), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var event Event
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				continue
			}
			events = append(events, event)
		}
	}
	if len(events) > n {
		events = events[len(events)-n:]
	}
	return events, nil
}

// logFiles orders the current log after its rotated backups (highest, oldest
// suffix first) so callers can read history chronologically.
func logFiles(path string) []string {
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		return []string{path}
	}
	base := filepath.Base(path)
	type backup struct {
		index int
		path  string
	}
	var backups []backup
	for _, entry := range entries {
		name := entry.Name()
		if name == base || entry.IsDir() {
			continue
		}
		suffix, ok := strings.CutPrefix(name, base+".")
		if !ok || suffix == "" {
			continue
		}
		index, err := strconv.Atoi(suffix)
		if err != nil {
			continue
		}
		backups = append(backups, backup{index: index, path: filepath.Join(filepath.Dir(path), name)})
	}
	sort.Slice(backups, func(i, j int) bool { return backups[i].index > backups[j].index })
	files := make([]string, 0, len(backups)+1)
	for _, backup := range backups {
		files = append(files, backup.path)
	}
	return append(files, path)
}
