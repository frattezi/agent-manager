package a2ui

import (
	"strings"
	"testing"

	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	"github.com/charmbracelet/x/ansi"
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

// A form keeps a gutter beside each field, fills the focused one, and says
// which field and choice each line shows so a click lands on it.
func TestRenderFormMarksTheFocusAndTargetsEachLine(t *testing.T) {
	doc := fixture(t, "round")
	lines, targets := RenderForm(doc, 60, Focus{Field: 3}, advanced)
	want := strings.TrimPrefix(`
╭──────────────────────────────────────────────────────────╮
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
│   Anything the choices do not cover                      │
│   > ▏_______________________                             │
│ [ Send answers ]                                         │
╰──────────────────────────────────────────────────────────╯`, "\n")
	if got := strings.Join(Texts(lines), "\n"); got != want {
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
	assertWithin(t, strings.Join(Texts(lines), "\n"), 60)
}

func TestRenderFormPutsTheCursorOnAMultipleChoice(t *testing.T) {
	doc := fixture(t, "every")
	lines, _ := RenderForm(doc, 30, Focus{Field: 3, Option: 2}, advanced)
	got := strings.Join(Texts(lines), "\n")
	for _, want := range []string{"  Tools", "    [x] claude", "    [ ] codex", "    [x] gemini"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
}

// Reading the document plainly draws no gutter, so surface show is
// unchanged by the form.
func TestRenderDrawsNoGutter(t *testing.T) {
	if got := Render(fixture(t, "round"), 80, advanced); strings.Contains(got, "› ") || strings.Contains(got, "│   1. Which") {
		t.Fatalf("plain render carries a form gutter:\n%s", got)
	}
}

func TestAnEmptyFocusedFieldShowsWhereTypingLands(t *testing.T) {
	doc := parse(t, `{"id":"root","component":"TextField","label":"Name"}`)
	lines, _ := RenderForm(doc, 12, Focus{Field: 1}, advanced)
	assertLines(t, lines, []string{
		`*body"  " *label"Name" *body"` + pad(6) + `"`,
		`*body"  " *muted"> " *focused"▏" *muted"_______"`,
	})
	lines, _ = RenderForm(doc, 12, Focus{}, advanced)
	assertLines(t, lines, []string{
		`body"  " label"Name"`,
		`body"  " muted"> ________"`,
	})
}

func TestCompatibleModeDrawsTheCaretInASCII(t *testing.T) {
	for _, value := range []string{"", "Ada"} {
		doc := parse(t, `{"id":"root","component":"TextField","label":"Name","value":"`+value+`"}`)
		lines, _ := RenderForm(doc, 30, Focus{Field: 1}, compatible)
		var caret []string
		for _, s := range lines[len(lines)-1] {
			if s.Role == RoleFocused {
				caret = append(caret, s.Text)
			}
		}
		if len(caret) != 1 || caret[0] != "|" {
			t.Errorf("value %q: focused text = %q, want one ASCII caret", value, caret)
		}
	}
}

// Focus arriving on a field fills it and adds its caret, and nothing else:
// the input keeps its width and its lines, so what follows stays put.
func TestFocusMovesNothingInOrAroundAField(t *testing.T) {
	for _, value := range []string{"", "Ada", "abcdefgh", "a value that wraps past the end"} {
		doc := parse(t, `{"id":"root","component":"Column","children":["q","after"]},
			{"id":"q","component":"TextField","label":"Name","value":"`+value+`"},
			{"id":"after","component":"Text","text":"below"}`)
		for _, chrome := range []presentation.ChromeSet{advanced, compatible} {
			focused, _ := RenderForm(doc, 12, Focus{Field: 1}, chrome)
			unfocused, _ := RenderForm(doc, 12, Focus{}, chrome)
			if len(focused) != len(unfocused) {
				t.Fatalf("value %q: %d lines focused, %d unfocused", value, len(focused), len(unfocused))
			}
			input := len(focused) - 2
			caret := ansi.StringWidth(chrome.CaretGlyph)
			if f, u := strings.TrimRight(focused[input].Text(), " "), unfocused[input].Text(); value == "" && ansi.StringWidth(f) != ansi.StringWidth(u) {
				t.Errorf("value %q: empty input is %d wide focused, %d unfocused", value, ansi.StringWidth(f), ansi.StringWidth(u))
			} else if value != "" && f != u+chrome.CaretGlyph {
				t.Errorf("value %q: focused input %q is not %q with a caret of %d", value, f, u, caret)
			}
			for i, l := range focused {
				if w := l.Width(); w > 12 {
					t.Errorf("value %q: line %d is %d wide", value, i, w)
				}
			}
			if focused[len(focused)-1].Text() != unfocused[len(unfocused)-1].Text() {
				t.Errorf("value %q: the line below moved", value)
			}
		}
	}
}

func TestPlainRenderingHasNoCaret(t *testing.T) {
	for _, chrome := range []presentation.ChromeSet{advanced, compatible} {
		for _, doc := range []string{
			`{"id":"root","component":"TextField","label":"Name"}`,
			`{"id":"root","component":"TextField","label":"Name","value":"Ada"}`,
		} {
			if got := Render(parse(t, doc), 30, chrome); strings.Contains(got, chrome.CaretGlyph) {
				t.Errorf("plain output carries a caret: %q", got)
			}
		}
	}
	if got, want := Render(parse(t, `{"id":"root","component":"TextField","label":"Name"}`), 30, advanced), "Name\n> "+strings.Repeat("_", 24); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
