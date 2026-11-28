package a2ui

import (
	"fmt"
	"strings"
	"testing"
)

// describe names each segment of a line by its role, in order, so a test
// reads what the text is and never how it is painted. A segment of the
// focused control is marked with a leading *.
func describe(l Line) string {
	parts := make([]string, len(l))
	for i, s := range l {
		fill := ""
		if s.Filled {
			fill = "*"
		}
		parts[i] = fmt.Sprintf("%s%s%q", fill, s.Role, s.Text)
	}
	return strings.Join(parts, " ")
}

func describeAll(lines []Line) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = describe(l)
	}
	return out
}

func pad(n int) string { return strings.Repeat(" ", n) }

func assertLines(t *testing.T, got []Line, want []string) {
	t.Helper()
	described := describeAll(got)
	if strings.Join(described, "\n") != strings.Join(want, "\n") {
		t.Fatalf("lines:\n%s\nwant\n%s", strings.Join(described, "\n"), strings.Join(want, "\n"))
	}
}

func TestVariantsTakeTheirRoles(t *testing.T) {
	doc := parse(t, `{"id":"root","component":"Column","children":["h1","h2","h3","caption","body","primary","plain"]},
		{"id":"h1","component":"Text","text":"One","variant":"h1"},
		{"id":"h2","component":"Text","text":"Two","variant":"h2"},
		{"id":"h3","component":"Text","text":"Three","variant":"h3"},
		{"id":"caption","component":"Text","text":"Aside","variant":"caption"},
		{"id":"body","component":"Text","text":"Prose"},
		{"id":"primary","component":"Button","child":"send","variant":"primary"},
		{"id":"send","component":"Text","text":"Send"},
		{"id":"plain","component":"Button","child":"skip"},
		{"id":"skip","component":"Text","text":"Skip"}`)
	lines, _ := RenderForm(doc, 40, Focus{}, advanced)
	assertLines(t, lines, []string{
		`title"One"`,
		`heading"Two"`,
		`heading"Three"`,
		`muted"Aside"`,
		`body"Prose"`,
		`accent"[ Send ]"`,
		`body"[ Skip ]"`,
	})
}

// Every variant the table maps belongs to a component the subset draws, so
// an entry cannot sit there unread.
func TestVariantRolesNameComponentsOfTheSubset(t *testing.T) {
	for name := range variantRoles {
		if _, ok := subset[name]; !ok {
			t.Errorf("variantRoles maps %s, which the subset does not draw", name)
		}
	}
}

// A round's lines say which text is the title, which the questions, which
// choice is selected and which control the focus is on.
func TestRenderFormGivesARoundItsRoles(t *testing.T) {
	lines, _ := RenderForm(fixture(t, "round"), 60, Focus{Field: 3}, advanced)
	rule := strings.Repeat("─", 58)
	assertLines(t, lines, []string{
		`rule"╭` + rule + `╮"`,
		`rule"│ " heading"Grilling round 1 of 3: where retained rounds live" body"` + pad(7) + `" rule" │"`,
		`rule"│ " body"  " label"1. Which package owns the A2UI format?" body"` + pad(16) + `" rule" │"`,
		`rule"│ " body"    " selected"(*) " body"internal/ui/a2ui, boundary-restricted like` + pad(6) + `" rule" │"`,
		`rule"│ " body"        internal/ui/runs` + pad(32) + `" rule" │"`,
		`rule"│ " body"    ( ) internal/ui/surface` + pad(29) + `" rule" │"`,
		`rule"│ " body"  " label"2. How many surfaces does a session keep?" body"` + pad(13) + `" rule" │"`,
		`rule"│ " body"    " selected"(*) " body"A named cap` + pad(37) + `" rule" │"`,
		`rule"│ " body"    ( ) All of them` + pad(37) + `" rule" │"`,
		`rule"│ " body"    ( ) A Settings row` + pad(34) + `" rule" │"`,
		`rule"│ ` + strings.Repeat("─", 56) + ` │"`,
		`rule"│ " *body"  " *label"Anything the choices do not cover" *body"` + pad(21) + `" rule" │"`,
		`rule"│ " *body"  " *muted"> " *focused"▏" *muted"` + strings.Repeat("_", 23) + `" *body"` + pad(28) + `" rule" │"`,
		`rule"│ " accent"[ Send answers ]" body"` + pad(40) + `" rule" │"`,
		`rule"╰` + rule + `╯"`,
	})
}

func TestAFocusedValueCarriesItsCaret(t *testing.T) {
	doc := parse(t, `{"id":"root","component":"TextField","label":"Name","value":"Ada"}`)
	lines, _ := RenderForm(doc, 30, Focus{Field: 1}, advanced)
	assertLines(t, lines, []string{
		`*body"  " *label"Name" *body"` + pad(24) + `"`,
		`*body"  " *muted"> " *value"Ada" *focused"▏" *body"` + pad(22) + `"`,
	})
	lines, _ = RenderForm(doc, 30, Focus{}, advanced)
	assertLines(t, lines, []string{
		`body"  " label"Name"`,
		`body"  " muted"> " value"Ada"`,
	})
}

// A choice keeps the option under the cursor apart from the ones that are
// selected, and both apart from the rest.
func TestTheCursorOptionIsFocusedAndTheChosenOnesSelected(t *testing.T) {
	doc := parse(t, `{"id":"root","component":"Column","children":["tools"]},
		{"id":"tools","component":"ChoicePicker","label":"Tools","variant":"multipleSelection",
		"options":[{"label":"claude","value":"claude"},{"label":"codex","value":"codex"},{"label":"gemini","value":"gemini"}],"value":["claude"]}`)
	lines, _ := RenderForm(doc, 30, Focus{Field: 1, Option: 2}, advanced)
	assertLines(t, lines, []string{
		`*body"  " *label"Tools" *body"` + pad(23) + `"`,
		`*body"    " *selected"[x] " *body"claude` + pad(16) + `"`,
		`*body"    [ ] " *focused"codex" *body"` + pad(17) + `"`,
		`*body"    [ ] gemini` + pad(16) + `"`,
	})
}

// A single choice puts its cursor on the chosen option, which then reads
// as both.
func TestASingleChoiceCursorSitsOnTheChosenOption(t *testing.T) {
	doc := parse(t, `{"id":"root","component":"ChoicePicker","label":"Pick","variant":"mutuallyExclusive",
		"options":[{"label":"one","value":"one"},{"label":"two","value":"two"}],"value":["two"]}`)
	lines, _ := RenderForm(doc, 20, Focus{Field: 1, Option: 2}, advanced)
	assertLines(t, lines, []string{
		`*body"  " *label"Pick" *body"` + pad(14) + `"`,
		`*body"    ( ) one` + pad(9) + `"`,
		`*body"    " *selected"(*) " *focused"two" *body"` + pad(9) + `"`,
	})
}

// Only the focused field is filled, and no line carries a gutter mark.
func TestOnlyTheFocusedFieldIsFilled(t *testing.T) {
	lines, targets := RenderForm(fixture(t, "round"), 60, Focus{Field: 2, Option: 1}, advanced)
	for i, l := range lines {
		for _, s := range l {
			if s.Role != RoleRule && s.Filled != (targets[i].Field == 2) {
				t.Errorf("line %d segment %s%q filled=%v", i, s.Role, s.Text, s.Filled)
			}
			if strings.Contains(s.Text, "›") {
				t.Errorf("line %d carries a gutter mark: %q", i, l.Text())
			}
		}
	}
}

func TestACheckBoxSetsItsMarkBesideItsLabel(t *testing.T) {
	doc := parse(t, `{"id":"root","component":"CheckBox","label":"Agree","value":true}`)
	assertLines(t, segments(pen{chrome: advanced}.draw(doc.Root, 30)), []string{`selected"[x] " label"Agree"`})
}

// Side by side, each column keeps its own roles, and the line ends where
// its last text does.
func TestARowKeepsEachColumnsRoles(t *testing.T) {
	doc := parse(t, `{"id":"root","component":"Row","children":["a","b"]},
		{"id":"a","component":"Text","text":"Left","variant":"h1"},
		{"id":"b","component":"Text","text":"Right","variant":"caption"}`)
	lines, _ := RenderForm(doc, 50, Focus{}, advanced)
	assertLines(t, lines, []string{`title"Left" body"` + pad(22) + `" muted"Right"`})
}

func TestAComponentOutsideTheSubsetCarriesTheGapRole(t *testing.T) {
	lines, _ := RenderForm(fixture(t, "unknown"), 80, Focus{}, advanced)
	found := false
	for _, l := range lines {
		for _, s := range l {
			if strings.Contains(s.Text, "skipped ") {
				found = true
				if s.Role != RoleGap {
					t.Errorf("%q carries %s, want gap", s.Text, s.Role)
				}
			}
		}
	}
	if !found {
		t.Fatalf("no gap in\n%s", strings.Join(describeAll(lines), "\n"))
	}
}

// The segments are text an agent wrote and the furniture around it, never
// terminal escapes, and they read back as the plain rendering does.
func TestSegmentsFlattenToThePlainRendering(t *testing.T) {
	for _, name := range []string{"round", "every", "unknown"} {
		doc := fixture(t, name)
		lines := segments(pen{chrome: advanced}.draw(doc.Root, 60))
		if got := strings.Join(Texts(lines), "\n"); got != Render(doc, 60, advanced) {
			t.Errorf("%s flattens to\n%s\nwant\n%s", name, got, Render(doc, 60, advanced))
		}
		form, _ := RenderForm(doc, 60, Focus{Field: 1, Option: 1}, advanced)
		for _, l := range append(lines, form...) {
			for _, s := range l {
				if s.Text == "" || strings.ContainsRune(s.Text, '\x1b') {
					t.Errorf("%s has segment %s%q", name, s.Role, s.Text)
				}
			}
		}
	}
}
