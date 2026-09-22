package main

import (
	"strings"
	"testing"
	"time"

	"github.com/wesleyxmns/sopro/internal/audit"
)

func TestLookupEnvPrefersEnvOverConfigFile(t *testing.T) {
	previous := configFile
	defer func() { configFile = previous }()
	configFile = map[string]string{"SOPRO_TEST_ONLY_FILE": "from-file", "SOPRO_TEST_BOTH": "from-file"}

	t.Setenv("SOPRO_TEST_BOTH", "from-env")
	if value, ok := lookupEnv("SOPRO_TEST_BOTH"); !ok || value != "from-env" {
		t.Fatalf("lookupEnv = %q, %v; want from-env", value, ok)
	}
	if value, ok := lookupEnv("SOPRO_TEST_ONLY_FILE"); !ok || value != "from-file" {
		t.Fatalf("lookupEnv = %q, %v; want from-file", value, ok)
	}
	if _, ok := lookupEnv("SOPRO_TEST_MISSING"); ok {
		t.Fatal("lookupEnv matched a missing key")
	}
}

func TestFormatAuditEvent(t *testing.T) {
	finished := time.Date(2026, 9, 22, 10, 11, 22, 0, time.UTC)
	formatted := formatAuditEvent(audit.Event{
		Action: "clean-cache-all", PID: 123, FinishedAt: finished,
		Success: true, ReclaimedBytes: 4 * 1024 * 1024 * 1024,
	})
	for _, expected := range []string{"2026-09-22 10:11:22", "clean-cache-all", "pid 123", "4.00 GB", "ok"} {
		if !strings.Contains(formatted, expected) {
			t.Fatalf("formatted = %q; want %q", formatted, expected)
		}
	}

	failed := formatAuditEvent(audit.Event{Action: "kill", FinishedAt: finished, Success: false, Error: "boom"})
	if !strings.Contains(failed, "FALHA") || !strings.Contains(failed, "boom") {
		t.Fatalf("formatted = %q; want failure with reason", failed)
	}
}
