package surfaces

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YoanWai/agent-manager/internal/ui/a2ui"
	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	tea "github.com/charmbracelet/bubbletea"
)

type fakeRoot struct {
	sent *[]AnswerRequest
	mode presentation.GlyphMode
}

func (fakeRoot) ChromeRows([][2]string) int          { return 6 }
func (r fakeRoot) GlyphMode() presentation.GlyphMode { return r.mode }
func (r fakeRoot) Send(request AnswerRequest) tea.Cmd {
	if r.sent != nil {
		*r.sent = append(*r.sent, request)
	}
	return nil
}

type fakeHost struct {
	cols, rows int
	popped     int
	body       string
	hint       [][2]string
}

func (h *fakeHost) CardSized(_ int, title, body string, hint [][2]string) string {
	h.body, h.hint = body, hint
	return title + "\n" + body
}
func (h *fakeHost) Size() (int, int) { return h.cols, h.rows }
func (h *fakeHost) Pop()             { h.popped++ }

func roundDocument(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "round.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func openRound(t *testing.T) (*Card, *fakeHost) {
	t.Helper()
	card, err := Open(fakeRoot{}, Stored{ID: "round-1", SessionID: "alpha", Document: roundDocument(t)}, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	return card, &fakeHost{cols: 100, rows: 40}
}

func press(c *Card, h *fakeHost, keys ...tea.KeyMsg) {
	for _, k := range keys {
		c.Key(h, k)
	}
}

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

var (
	tab      = tea.KeyMsg{Type: tea.KeyTab}
	shiftTab = tea.KeyMsg{Type: tea.KeyShiftTab}
	right    = tea.KeyMsg{Type: tea.KeyRight}
	left     = tea.KeyMsg{Type: tea.KeyLeft}
	space    = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	esc      = tea.KeyMsg{Type: tea.KeyEsc}
)

func selected(n *a2ui.Node) []string {
	var chosen []string
	for _, o := range n.Options {
		if o.Selected {
			chosen = append(chosen, o.Value)
		}
	}
	return chosen
}

func TestTheCardOpensOnTheFirstQuestionWithItsRecommendationChosen(t *testing.T) {
	card, h := openRound(t)
	if card.Focus() != (a2ui.Focus{Field: 1, Option: 1}) {
		t.Fatalf("focus = %+v", card.Focus())
	}
	view := card.View(h, "")
	for _, want := range []string{"alpha is asking", "  1. Which package owns the A2UI format?", "(*) internal/ui/a2ui", "(*) A named cap"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q in\n%s", want, view)
		}
	}
	if card.SessionID() != "alpha" || card.SurfaceID() != "round-1" {
		t.Fatalf("card is for %s/%s", card.SessionID(), card.SurfaceID())
	}
}

func TestKeysMoveBetweenQuestionsStepAChoiceAndTypeIntoAField(t *testing.T) {
	card, h := openRound(t)
	press(card, h, tab, right, right, right)
	doc := card.Document()
	fields := a2ui.Fields(&doc)
	if got := selected(fields[1]); len(got) != 1 || got[0] != "setting" {
		t.Fatalf("q2 after stepping right = %v, want [setting]", got)
	}
	press(card, h, left)
	if got := selected(fields[1]); got[0] != "all" {
		t.Fatalf("q2 after stepping left = %v, want [all]", got)
	}
	press(card, h, shiftTab, right)
	if got := selected(fields[0]); got[0] != "surface" {
		t.Fatalf("q1 = %v", got)
	}
	press(card, h, tab, tab, runes("keep"), space, runes("all"), tea.KeyMsg{Type: tea.KeyBackspace})
	if fields[2].Value != "keep al" {
		t.Fatalf("field = %q", fields[2].Value)
	}
	if view := card.View(h, ""); !strings.Contains(view, "> keep al"+presentation.ChromeFor(presentation.GlyphModeAdvanced).CaretGlyph) {
		t.Fatalf("the caret is not drawn:\n%s", view)
	}
	if h.popped != 0 {
		t.Fatal("a key in the card closed it")
	}
}

func TestClicksFocusAQuestionPickAChoiceAndFlipACheckBox(t *testing.T) {
	card, h := openRound(t)
	card.View(h, "")
	line := lineOf(t, h.body, "All of them")
	card.Click(h, line)
	doc := card.Document()
	fields := a2ui.Fields(&doc)
	if card.Focus().Field != 2 || selected(fields[1])[0] != "all" {
		t.Fatalf("after the click focus = %+v, q2 = %v", card.Focus(), selected(fields[1]))
	}
	card.View(h, "")
	card.Click(h, lineOf(t, h.body, "Anything the choices"))
	if card.Focus().Field != 3 {
		t.Fatalf("clicking the field focused %+v", card.Focus())
	}
	card.Click(h, 0)
	if card.Focus().Field != 3 {
		t.Fatal("a click on the card's edge moved the focus")
	}

	boxes, err := Open(fakeRoot{}, Stored{ID: "b", SessionID: "alpha", Document: document(`{"id":"root","component":"Column","children":["c"]},{"id":"c","component":"CheckBox","label":"Ship it","value":false}`)}, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	boxes.View(h, "")
	boxes.Click(h, lineOf(t, h.body, "Ship it"))
	boxes.View(h, "")
	if !strings.Contains(h.body, "[x] Ship it") {
		t.Fatalf("the click did not check the box:\n%s", h.body)
	}
	press(boxes, h, space)
	boxes.View(h, "")
	if !strings.Contains(h.body, "[ ] Ship it") {
		t.Fatalf("space did not uncheck the box:\n%s", h.body)
	}
}

func TestEscClosesTheCard(t *testing.T) {
	card, h := openRound(t)
	if handled, _ := card.Key(h, esc); !handled || h.popped != 1 {
		t.Fatalf("esc handled %v, popped %d", handled, h.popped)
	}
	if card.Dismissible() {
		t.Fatal("a stray click would drop what the owner typed")
	}
}

// The card is modal: a list key reaches nothing beneath it.
func TestTheCardTakesEveryKey(t *testing.T) {
	card, h := openRound(t)
	for _, k := range []tea.KeyMsg{runes("q"), runes("d"), runes("x"), {Type: tea.KeyEnter}} {
		if handled, _ := card.Key(h, k); !handled {
			t.Errorf("%s fell through the card", k)
		}
	}
}

func TestAComponentThisBuildCannotDrawOpensAsALabelledGap(t *testing.T) {
	card, err := Open(fakeRoot{}, Stored{ID: "g", SessionID: "alpha", Document: document(`{"id":"root","component":"Column","children":["chart","q"]},{"id":"chart","component":"Chart"},{"id":"q","component":"TextField","label":"Why?"}`)}, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	h := &fakeHost{cols: 100, rows: 40}
	if view := card.View(h, ""); !strings.Contains(view, `[skipped Chart "chart": not in the subset this build`) || !strings.Contains(view, "  Why?") {
		t.Fatalf("view:\n%s", view)
	}
}

func TestADocumentThisBuildRefusesDoesNotOpen(t *testing.T) {
	if _, err := Open(fakeRoot{}, Stored{ID: "v", SessionID: "alpha", Document: `[{"version":"v9.9"}]`}, "alpha"); err == nil {
		t.Fatal("a refused document opened")
	}
}

// A round taller than the card scrolls to keep the focused question shown,
// and the wheel scrolls it too.
func TestATallRoundFollowsTheFocusAndScrolls(t *testing.T) {
	card, _ := openRound(t)
	h := &fakeHost{cols: 100, rows: 12}
	card.View(h, "")
	if strings.Contains(h.body, "Anything the choices") {
		t.Fatalf("a short card shows the whole round:\n%s", h.body)
	}
	press(card, h, tab, tab)
	card.View(h, "")
	if !strings.Contains(h.body, "Anything the choices") {
		t.Fatalf("the focused field is off the card:\n%s", h.body)
	}
	card.Wheel(h, tea.KeyMsg{Type: tea.KeyUp})
	card.Wheel(h, tea.KeyMsg{Type: tea.KeyUp})
	before := h.body
	card.View(h, "")
	if h.body == before {
		t.Fatal("the wheel did not scroll the card")
	}
}

func TestTheFootNamesTheKeysOfTheFocusedField(t *testing.T) {
	card, h := openRound(t)
	card.View(h, "")
	if !hinted(h.hint, "←/→") || !hinted(h.hint, "tab") || !hinted(h.hint, "esc") {
		t.Fatalf("hint = %v", h.hint)
	}
	press(card, h, tab, tab)
	card.View(h, "")
	if !hinted(h.hint, "⌫") || hinted(h.hint, "←/→") {
		t.Fatalf("hint on the field = %v", h.hint)
	}
}

func hinted(hint [][2]string, key string) bool {
	for _, pair := range hint {
		if pair[0] == key {
			return true
		}
	}
	return false
}

func document(components string) string {
	return `[{"version":"v0.9","createSurface":{"surfaceId":"s","catalogId":"basic"}},` +
		`{"version":"v0.9","updateComponents":{"surfaceId":"s","components":[` + components + `]}}]`
}

func lineOf(t *testing.T, body, text string) int {
	t.Helper()
	for i, line := range strings.Split(body, "\n") {
		if strings.Contains(line, text) {
			return i
		}
	}
	t.Fatalf("%q is not on the card:\n%s", text, body)
	return -1
}

// The card hands the renderer the set the owner's glyph mode resolves to:
// rounded corners like every other card, or ASCII throughout in Compatible.
func TestTheCardDrawsInTheOwnersGlyphMode(t *testing.T) {
	for _, tc := range []struct {
		mode        presentation.GlyphMode
		top, bottom string
	}{
		{presentation.GlyphModeAdvanced, "╭─", "╰─"},
		{presentation.GlyphModeBasic, "+-", "+-"},
	} {
		card, err := Open(fakeRoot{mode: tc.mode}, Stored{ID: "round-1", SessionID: "alpha", Document: roundDocument(t)}, "alpha")
		if err != nil {
			t.Fatal(err)
		}
		lines, _ := card.render(60)
		text := strings.Join(a2ui.Texts(lines), "\n")
		if first, last := a2ui.Texts(lines)[0], a2ui.Texts(lines)[len(lines)-1]; !strings.HasPrefix(first, tc.top) || !strings.HasPrefix(last, tc.bottom) {
			t.Errorf("%s frame is %q … %q", tc.mode, first, last)
		}
		if tc.mode == presentation.GlyphModeBasic {
			for _, r := range text {
				if r >= 0x2500 && r <= 0x259f {
					t.Errorf("Compatible card draws %q:\n%s", r, text)
					break
				}
			}
		}
	}
}
