package surfaces

import (
	"fmt"
	"strings"
	"testing"
)

// tallRound is a round of text fields longer than a short card, its last
// control a text field with a button after it.
func tallRound(t *testing.T, label string) *Card {
	t.Helper()
	ids := []string{`"title"`}
	components := []string{`{"id":"title","component":"Text","text":"A long round","variant":"h2"}`}
	for i := 1; i <= 5; i++ {
		ids = append(ids, fmt.Sprintf(`"q%d"`, i))
		components = append(components, fmt.Sprintf(`{"id":"q%d","component":"TextField","label":"Question %d"}`, i, i))
	}
	ids = append(ids, `"last"`, `"send"`)
	components = append(components,
		fmt.Sprintf(`{"id":"last","component":"TextField","label":%q}`, label),
		`{"id":"send","component":"Button","child":"send-label","variant":"primary","action":{"event":{"name":"submit"}}}`,
		`{"id":"send-label","component":"Text","text":"Send answers"}`)
	root := `{"id":"root","component":"Column","children":[` + strings.Join(ids, ",") + `]}`
	card, err := Open(fakeRoot{}, Stored{ID: "tall", SessionID: "alpha", Document: document(root + "," + strings.Join(components, ","))}, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	return card
}

const (
	moreAbove = "more above"
	moreBelow = "more below"
)

// inputShown is whether the body draws the field labelled label with its
// input line right under it, clear of the indicators.
func inputShown(body, label string) bool {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.Contains(line, label) && i+1 < len(lines) {
			return strings.Contains(lines[i+1], "> ")
		}
	}
	return false
}

func TestMovementWrapsAtBothEnds(t *testing.T) {
	card := tallRound(t, "Last question")
	h := &fakeHost{cols: 100, rows: 14}
	press(card, h, shiftTab)
	if card.Focus().Field != 6 {
		t.Fatalf("shift+tab from the first control focused %+v", card.Focus())
	}
	card.View(h, "")
	if !strings.Contains(h.body, moreAbove) || strings.Contains(h.body, moreBelow) {
		t.Fatalf("on the last control the indicators read:\n%s", h.body)
	}
	press(card, h, tab)
	if card.Focus().Field != 1 {
		t.Fatalf("tab from the last control focused %+v", card.Focus())
	}
	card.View(h, "")
	if strings.Contains(h.body, moreAbove) || !strings.Contains(h.body, moreBelow) || !strings.HasPrefix(h.body, "A long round") {
		t.Fatalf("back on the first control the indicators read:\n%s", h.body)
	}
}

func TestTheFootFollowsFocusAsItWraps(t *testing.T) {
	card, h := openRound(t)
	press(card, h, shiftTab)
	card.View(h, "")
	if card.Focus().Field != 3 || !hinted(h.hint, "⌫") || hinted(h.hint, "←/→") {
		t.Fatalf("wrapped to %+v, hint = %v", card.Focus(), h.hint)
	}
	press(card, h, tab)
	card.View(h, "")
	if card.Focus().Field != 1 || !hinted(h.hint, "←/→") || hinted(h.hint, "⌫") {
		t.Fatalf("wrapped to %+v, hint = %v", card.Focus(), h.hint)
	}
}

func TestFocusBringsTheWholeFieldIntoView(t *testing.T) {
	card := tallRound(t, "Last question")
	h := &fakeHost{cols: 100, rows: 14}
	for i, label := range []string{"Question 2", "Question 3", "Question 4", "Question 5", "Last question"} {
		press(card, h, tab)
		card.View(h, "")
		if !inputShown(h.body, label) {
			t.Fatalf("tab %d leaves %q without its input line:\n%s", i+1, label, h.body)
		}
	}
	press(card, h, runes("hello"))
	card.View(h, "")
	if !strings.Contains(h.body, "> hello_") {
		t.Fatalf("what is typed is off the card:\n%s", h.body)
	}
	for i, label := range []string{"Question 5", "Question 4", "Question 3", "Question 2", "Question 1"} {
		press(card, h, shiftTab)
		card.View(h, "")
		if !inputShown(h.body, label) {
			t.Fatalf("shift+tab %d leaves %q without its input line:\n%s", i+1, label, h.body)
		}
	}
}

func TestAFieldTallerThanTheCardShowsItsInput(t *testing.T) {
	card := tallRound(t, strings.TrimSpace(strings.Repeat("a long question ", 80)))
	h := &fakeHost{cols: 100, rows: 14}
	press(card, h, shiftTab)
	card.View(h, "")
	lines := strings.Split(h.body, "\n")
	input := -1
	for i, line := range lines {
		if strings.Contains(line, "> _") {
			input = i
		}
	}
	if input < 1 || !strings.Contains(lines[input-1], "a long question") || !strings.Contains(lines[0], moreAbove) {
		t.Fatalf("the tall field's input is not on the card:\n%s", h.body)
	}
	press(card, h, runes("yes"))
	card.View(h, "")
	if !strings.Contains(h.body, "> yes_") {
		t.Fatalf("what is typed is off the card:\n%s", h.body)
	}
}

// A click on a field whose input is under the fold lands the card where tab
// would have.
func TestAClickFocusesAndScrollsAsTheKeyDoes(t *testing.T) {
	h := &fakeHost{cols: 100, rows: 13}
	clicked, keyed := tallRound(t, "Last question"), tallRound(t, "Last question")
	clicked.View(h, "")
	if inputShown(h.body, "Question 3") {
		t.Fatalf("the round fits before the click:\n%s", h.body)
	}
	clicked.Click(h, lineOf(t, h.body, "Question 3"))
	press(keyed, h, tab, tab)
	if clicked.Focus() != keyed.Focus() || clicked.offset != keyed.offset {
		t.Fatalf("click left %+v at %d, keys %+v at %d", clicked.Focus(), clicked.offset, keyed.Focus(), keyed.offset)
	}
	view := clicked.View(h, "")
	hint := h.hint
	if view != keyed.View(h, "") || fmt.Sprint(hint) != fmt.Sprint(h.hint) {
		t.Fatalf("the click and the keys draw different cards:\n%s\n---\n%s", view, keyed.View(h, ""))
	}
	if !inputShown(h.body, "Question 3") || !strings.Contains(h.body, moreAbove) || !strings.Contains(h.body, moreBelow) {
		t.Fatalf("after the click:\n%s", h.body)
	}
}

func TestAChoiceTallerThanTheCardKeepsItsCursorShown(t *testing.T) {
	var options []string
	for i := 1; i <= 15; i++ {
		options = append(options, fmt.Sprintf(`{"label":"option %02d","value":"o%d"}`, i, i))
	}
	card, err := Open(fakeRoot{}, Stored{ID: "c", SessionID: "alpha", Document: document(
		`{"id":"root","component":"Column","children":["pick"]},` +
			`{"id":"pick","component":"ChoicePicker","label":"Pick one","variant":"mutuallyExclusive","options":[` + strings.Join(options, ",") + `]}`)}, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	h := &fakeHost{cols: 100, rows: 14}
	for i := 2; i <= 15; i++ {
		press(card, h, right)
		card.View(h, "")
		if !strings.Contains(h.body, fmt.Sprintf("(*) option %02d", i)) {
			t.Fatalf("option %d is chosen off the card:\n%s", i, h.body)
		}
	}
	for i := 14; i >= 1; i-- {
		press(card, h, left)
		card.View(h, "")
		if !strings.Contains(h.body, fmt.Sprintf("(*) option %02d", i)) {
			t.Fatalf("option %d is chosen off the card:\n%s", i, h.body)
		}
	}
}
