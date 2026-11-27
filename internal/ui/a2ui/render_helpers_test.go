package a2ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/YoanWai/agent-manager/internal/ui/presentation"
)

func fixture(t *testing.T, name string) Document {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return doc
}

// surface wraps components in a one-surface document.
func surface(components string) string {
	return `[{"version":"v0.9","createSurface":{"surfaceId":"s","catalogId":"basic"}},` +
		`{"version":"v0.9","updateComponents":{"surfaceId":"s","components":[` + components + `]}}]`
}

func parse(t *testing.T, components string) Document {
	t.Helper()
	doc, err := Parse([]byte(surface(components)))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

var (
	advanced   = presentation.ChromeFor(presentation.GlyphModeAdvanced)
	compatible = presentation.ChromeFor(presentation.GlyphModeBasic)
)
