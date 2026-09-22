package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadParsesKeyValueFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sopro.conf")
	content := "# comment\n\nSOPRO_THEME=dark\nSOPRO_DAEMON_INTERVAL = \"5s\"\nEMPTY=\nbroken-line\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := file.Lookup("SOPRO_THEME"); value != "dark" {
		t.Fatalf("SOPRO_THEME = %q; want dark", value)
	}
	if value, _ := file.Lookup("SOPRO_DAEMON_INTERVAL"); value != "5s" {
		t.Fatalf("SOPRO_DAEMON_INTERVAL = %q; want 5s unquoted", value)
	}
	if value, ok := file.Lookup("EMPTY"); !ok || value != "" {
		t.Fatalf("EMPTY = %q, %v; want present and empty", value, ok)
	}
	if _, ok := file.Lookup("broken-line"); ok {
		t.Fatal("line without '=' must be ignored")
	}
}

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	file, err := Load(filepath.Join(t.TempDir(), "absent.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if len(file) != 0 {
		t.Fatalf("file = %+v; want empty", file)
	}
	if _, ok := file.Lookup("SOPRO_THEME"); ok {
		t.Fatal("empty file must not match lookups")
	}
}
