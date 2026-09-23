package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The 3D logo must not depend on the U+2591 glyph: terminal fonts often
// substitute shade characters from another face with mismatched metrics,
// which shears the rows. Shade pixels are dimmed full blocks instead.
func TestRenderLogoAvoidsShadeGlyph(t *testing.T) {
	model, _ := newTestModel()
	stripped := ansi.Strip(model.renderLogo())
	if strings.Contains(stripped, "░") {
		t.Fatal("rendered logo relies on the ░ glyph for shading")
	}
	if !strings.Contains(stripped, "████  ███") {
		t.Fatal("rendered logo lost the solid SOPRO letterforms")
	}
}
