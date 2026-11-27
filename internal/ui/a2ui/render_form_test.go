package a2ui

import (
	"strings"
	"testing"
)

func TestFieldsNumberTheAnsweredComponentsInDocumentOrder(t *testing.T) {
	doc := fixture(t, "every")
	var got []string
	for i, f := range Fields(&doc) {
		if f.Field != i+1 {
			t.Errorf("%s is field %d, want %d", f.ID, f.Field, i+1)
		}
		got = append(got, f.ID)
	}
	if strings.Join(got, ",") != "name,agree,tools" {
		t.Fatalf("fields = %v", got)
	}
	if doc.Root.Field != 0 {
		t.Fatalf("a layout is numbered %d", doc.Root.Field)
	}
}

// A form keeps a gutter beside each field, marks the focused one, and says
// which field and choice each line shows so a click lands on it.
func TestRenderFormMarksTheFocusAndTargetsEachLine(t *testing.T) {
	doc := fixture(t, "round")
	lines, targets := RenderForm(doc, 60, Focus{Field: 3})
	want := strings.TrimPrefix(`
+──────────────────────────────────────────────────────────+
│ Grilling round 1 of 3: where retained rounds live        │
│   1. Which package owns the A2UI format?                 │
│     (*) internal/ui/a2ui, boundary-restricted like       │
│         internal/ui/runs                                 │
│     ( ) internal/ui/surface                              │
│   2. How many surfaces does a session keep?              │
│     (*) A named cap                                      │
│     ( ) All of them                                      │
│     ( ) A Settings row                                   │
│ ──────────────────────────────────────────────────────── │
│ › Anything the choices do not cover                      │
│   > _                                                    │
│ [ Send answers ]                                         │
+──────────────────────────────────────────────────────────+`, "\n")
	if got := strings.Join(lines, "\n"); got != want {
		t.Fatalf("form:\n%s\nwant\n%s", got, want)
	}
	wantTargets := []Target{{}, {}, {1, 0}, {1, 1}, {1, 1}, {1, 2}, {2, 0}, {2, 1}, {2, 2}, {2, 3}, {}, {3, 0}, {3, 0}, {}, {}}
	if len(targets) != len(wantTargets) {
		t.Fatalf("targets = %v", targets)
	}
	for i := range targets {
		if targets[i] != wantTargets[i] {
			t.Errorf("line %d targets %v, want %v", i, targets[i], wantTargets[i])
		}
	}
	assertWithin(t, strings.Join(lines, "\n"), 60)
}

func TestRenderFormPutsTheCursorOnAMultipleChoice(t *testing.T) {
	doc := fixture(t, "every")
	lines, _ := RenderForm(doc, 30, Focus{Field: 3, Option: 2})
	got := strings.Join(lines, "\n")
	for _, want := range []string{"› Tools", "    [x] claude", "  › [ ] codex", "    [x] gemini"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
}

// Reading the document plainly draws no gutter, so surface show is
// unchanged by the form.
func TestRenderDrawsNoGutter(t *testing.T) {
	if got := Render(fixture(t, "round"), 80); strings.Contains(got, "› ") || strings.Contains(got, "│   1. Which") {
		t.Fatalf("plain render carries a form gutter:\n%s", got)
	}
}
