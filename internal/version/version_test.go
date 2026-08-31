package version

import (
	"strings"
	"testing"
)

func TestVersionInfo(t *testing.T) {
	info := Info()
	if !strings.Contains(info, "sopro") {
		t.Fatalf("expected version info to contain 'sopro', got: %s", info)
	}
	if !strings.Contains(info, "plataforma:") {
		t.Fatalf("expected version info to contain 'plataforma:', got: %s", info)
	}
}

func TestVersionShort(t *testing.T) {
	short := Short()
	if !strings.HasPrefix(short, "sopro ") {
		t.Fatalf("expected short version to start with 'sopro ', got: %s", short)
	}
}
