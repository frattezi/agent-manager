package ui

import (
	"strings"
	"testing"

	"github.com/YoanWai/agent-manager/internal/ui/a2ui"
	uisurfaces "github.com/YoanWai/agent-manager/internal/ui/surfaces"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func openTestCard(t *testing.T) (*Model, *uisurfaces.Card) {
	t.Helper()
	m, _ := askingModel(t)
	m.selectSessionRow(t, "alpha")
	m.openAgentSurface()
	card, ok := surfaceOf[*uisurfaces.Card](m)
	if !ok {
		t.Fatalf("no card; err %q", m.errBar.text)
	}
	return m, card
}

func chosen(card *uisurfaces.Card, field int) []string {
	doc := card.Document()
	var values []string
	for _, o := range a2ui.Fields(&doc)[field-1].Options {
		if o.Selected {
			values = append(values, o.Value)
		}
	}
	return values
}

func TestAClickOnTheCardPicksTheChoiceUnderIt(t *testing.T) {
	m, card := openTestCard(t)
	x, y := paintedCell(t, m, "All of them")
	m = leftClick(m, x, y)
	if got := chosen(card, 2); len(got) != 1 || got[0] != "all" || card.Focus().Field != 2 {
		t.Fatalf("after the click q2 = %v, focus %+v", got, card.Focus())
	}
	if top, _ := m.top(); top != card {
		t.Fatal("a click on the card closed it")
	}
}

func TestKeysThroughTheRootStepTheRoundAndTypeIntoItsField(t *testing.T) {
	m, card := openTestCard(t)
	for _, k := range []tea.KeyMsg{{Type: tea.KeyTab}, {Type: tea.KeyRight}, {Type: tea.KeyTab}, key("n"), key("o")} {
		m.handleKey(k)
	}
	if got := chosen(card, 2); got[0] != "all" {
		t.Fatalf("q2 = %v", got)
	}
	if view := ansi.Strip(preparedView(m)); !strings.Contains(view, "> no▏") {
		t.Fatalf("the typed answer is not drawn:\n%s", view)
	}
	if len(m.workspace.sessions) != 2 {
		t.Fatal("n reached the list under the card")
	}
}

func TestClickingTheCardsEscClosesIt(t *testing.T) {
	m, _ := openTestCard(t)
	x, y := paintedCell(t, m, "close, still unanswered")
	m = leftClick(m, x, y)
	if _, open := surfaceOf[*uisurfaces.Card](m); open {
		t.Fatal("clicking esc on the card's foot left it open")
	}
}

func TestAGapOpensAsALabelledPartOfTheCard(t *testing.T) {
	m := buildModel(t)
	m.layout.width, m.layout.height = 120, 40
	createSession(t, m, "alpha", t.TempDir(), "")
	doc := `[{"version":"v0.9","createSurface":{"surfaceId":"g","catalogId":"basic"}},` +
		`{"version":"v0.9","updateComponents":{"surfaceId":"g","components":[` +
		`{"id":"root","component":"Column","children":["chart","q"]},{"id":"chart","component":"Chart"},{"id":"q","component":"TextField","label":"Why?"}]}}]`
	publishTestSurface(t, m, "alpha", "g", doc, false)
	m.openAgentSurface()
	view := ansi.Strip(preparedView(m))
	if !strings.Contains(view, `[skipped Chart "chart"`) || !strings.Contains(view, "Why?") {
		t.Fatalf("view:\n%s", view)
	}
}
