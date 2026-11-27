package a2ui

import (
	"strings"
	"testing"

	"github.com/YoanWai/agent-manager/internal/ui/presentation"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderShowsARoundWithTheRecommendedChoicesSelected(t *testing.T) {
	for _, tc := range []struct {
		width int
		want  string
	}{
		{80, `
╭──────────────────────────────────────────────────────────────────────────────╮
│ Grilling round 1 of 3: where retained rounds live                            │
│ 1. Which package owns the A2UI format?                                       │
│   (*) internal/ui/a2ui, boundary-restricted like internal/ui/runs            │
│   ( ) internal/ui/surface                                                    │
│ 2. How many surfaces does a session keep?                                    │
│   (*) A named cap                                                            │
│   ( ) All of them                                                            │
│   ( ) A Settings row                                                         │
│ ──────────────────────────────────────────────────────────────────────────── │
│ Anything the choices do not cover                                            │
│ > ________________________                                                   │
│ [ Send answers ]                                                             │
╰──────────────────────────────────────────────────────────────────────────────╯`},
		{30, `
╭────────────────────────────╮
│ Grilling round 1 of 3:     │
│ where retained rounds live │
│ 1. Which package owns the  │
│ A2UI format?               │
│   (*) internal/ui/a2ui,    │
│       boundary-restricted  │
│       like                 │
│       internal/ui/runs     │
│   ( ) internal/ui/surface  │
│ 2. How many surfaces does  │
│ a session keep?            │
│   (*) A named cap          │
│   ( ) All of them          │
│   ( ) A Settings row       │
│ ────────────────────────── │
│ Anything the choices do    │
│ not cover                  │
│ > ________________________ │
│ [ Send answers ]           │
╰────────────────────────────╯`},
	} {
		got := Render(fixture(t, "round"), tc.width, advanced)
		if want := strings.TrimPrefix(tc.want, "\n"); got != want {
			t.Errorf("width %d:\n%s\nwant\n%s", tc.width, got, want)
		}
		assertWithin(t, got, tc.width)
	}
}

func TestRenderDrawsEveryComponentNested(t *testing.T) {
	for _, tc := range []struct {
		width int
		want  string
	}{
		{80, `
Every component
╭──────────────────────────────────────────────────────────────────────────────╮
│ Name                                   Tools                                 │
│ > Ada                                    [x] claude                          │
│ [x] Agree                                [ ] codex                           │
│                                          [x] gemini                          │
│                                        [ OK ]                                │
╰──────────────────────────────────────────────────────────────────────────────╯
first item
second item
────────────────────────────────────────────────────────────────────────────────`},
		{30, `
Every component
╭────────────────────────────╮
│ Name                       │
│ > Ada                      │
│ [x] Agree                  │
│ Tools                      │
│   [x] claude               │
│   [ ] codex                │
│   [x] gemini               │
│ [ OK ]                     │
╰────────────────────────────╯
first item
second item
──────────────────────────────`},
	} {
		got := Render(fixture(t, "every"), tc.width, advanced)
		if want := strings.TrimPrefix(tc.want, "\n"); got != want {
			t.Errorf("width %d:\n%s\nwant\n%s", tc.width, got, want)
		}
		assertWithin(t, got, tc.width)
	}
}

func TestRenderEveryComponentOfTheSubsetIsInTheFixture(t *testing.T) {
	seen := map[string]bool{}
	var walk func(Node)
	walk = func(n Node) {
		seen[n.Type] = true
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(fixture(t, "every").Root)
	for _, name := range Subset() {
		if !seen[name] {
			t.Errorf("testdata/every.json has no %s", name)
		}
	}
}

func TestRenderLabelsAComponentOutsideTheSubsetAndDrawsTheRest(t *testing.T) {
	got := Render(fixture(t, "unknown"), 80, advanced)
	want := "Before the slider\n[skipped Slider \"volume\": not in the subset this build draws]\nAfter the slider"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	assertWithin(t, Render(fixture(t, "unknown"), 30, advanced), 30)
}

func TestRenderLabelsTemplateChildren(t *testing.T) {
	doc := parse(t, `{"id":"root","component":"List","children":{"componentId":"item","path":"/items"}}`)
	if got, want := Render(doc, 80, advanced), "[skipped template: children from a data template are not drawn by this build]"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderSetsAHorizontalListSideBySide(t *testing.T) {
	doc := parse(t, `{"id":"root","component":"List","direction":"horizontal","children":["a","b"]},
		{"id":"a","component":"Text","text":"left"},{"id":"b","component":"Text","text":"right"}`)
	if got, want := Render(doc, 60, advanced), "left"+strings.Repeat(" ", 27)+"right"; got != want {
		t.Fatalf("wide = %q, want %q", got, want)
	}
	if got, want := Render(doc, 30, advanced), "left\nright"; got != want {
		t.Fatalf("narrow = %q, want %q", got, want)
	}
}

func TestRenderEscapesControlCharacters(t *testing.T) {
	doc := parse(t, `{"id":"root","component":"Text","text":"red \u001b[31mtext\u0007"}`)
	got := Render(doc, 80, advanced)
	if strings.ContainsAny(got, "\x1b\x07") || got != "red ^[[31mtext^G" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderTruncatesAButtonWiderThanTheSurface(t *testing.T) {
	doc := parse(t, `{"id":"root","component":"Button","child":"l"},{"id":"l","component":"Text","text":"Send every answer now"}`)
	if got, want := Render(doc, 12, advanced), "[ Send ever…"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderShowsAFilledTextField(t *testing.T) {
	doc := parse(t, `{"id":"root","component":"TextField","label":"Other","value":"keep both, the cap is fine"}`)
	if got, want := Render(doc, 20, advanced), "Other\n> keep both, the cap\n  is fine"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func assertWithin(t *testing.T, text string, width int) {
	t.Helper()
	for i, line := range strings.Split(text, "\n") {
		if w := ansi.StringWidth(line); w > width {
			t.Errorf("line %d is %d wide, past %d: %q", i, w, width, line)
		}
	}
}

func TestACompatibleSurfaceDrawsNoBoxCharacter(t *testing.T) {
	for _, name := range []string{"round", "every", "unknown"} {
		doc := fixture(t, name)
		form, _ := RenderForm(doc, 60, Focus{Field: 1, Option: 1}, compatible)
		drawn := Render(doc, 60, compatible) + "\n" + strings.Join(Texts(form), "\n")
		for _, r := range drawn {
			if r >= 0x2500 && r <= 0x259f {
				t.Fatalf("%s draws %q in Compatible mode:\n%s", name, r, drawn)
			}
		}
	}
}

func TestACompatibleCardIsASCIIEndToEnd(t *testing.T) {
	got := Render(fixture(t, "every"), 30, compatible)
	want := `Every component
+----------------------------+
| Name                       |
| > Ada                      |
| [x] Agree                  |
| Tools                      |
|   [x] claude               |
|   [ ] codex                |
|   [x] gemini               |
| [ OK ]                     |
+----------------------------+
first item
second item
------------------------------`
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

// Every piece of furniture is read from the set the caller resolved, so a
// set no mode ships shows up wherever the renderer draws one.
func TestFurnitureComesFromTheSetPassedIn(t *testing.T) {
	set := presentation.ChromeSet{
		TopLeftGlyph: "1", TopRightGlyph: "2", BottomLeftGlyph: "3", BottomRightGlyph: "4",
		EdgeGlyph: "=", SideGlyph: "!",
		ChoiceOnMark: "<on>", ChoiceOffMark: "<no>", CheckOnMark: "{y}", CheckOffMark: "{n}",
	}
	got := Render(fixture(t, "every"), 30, set) + "\n" + Render(fixture(t, "round"), 30, set)
	for _, want := range []string{"1====", "====2", "3====", "====4", "! Name", "{y} Agree", "{n} codex", "<on> A named cap", "<no> All of them", "\n" + strings.Repeat("=", 30)} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	for _, drawn := range []string{"─", "│", "+", "(*)", "[x]"} {
		if strings.Contains(got, drawn) {
			t.Errorf("drew %q, which the set does not carry:\n%s", drawn, got)
		}
	}
}
