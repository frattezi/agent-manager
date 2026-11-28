package ui

import (
	"strings"
	"testing"

	uisurfaces "github.com/YoanWai/agent-manager/internal/ui/surfaces"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func focusAsking(t *testing.T) *Model {
	t.Helper()
	m, _ := askingModel(t)
	m.selectSessionRow(t, "alpha")
	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	*m = *updated.(*Model)
	m.drainEffects(t)
	if !m.focused() {
		t.Fatalf("enter did not focus, screen = %v", m.screenID())
	}
	return m
}

func TestFocusedAskingSessionNamesTheKeyAndTheKeyOpensTheCard(t *testing.T) {
	m := focusAsking(t)
	m.prepareFrame()
	if footer := ansi.Strip(m.viewFooter()); !strings.Contains(footer, "open what it asks") {
		t.Fatalf("footer = %q", footer)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyF2})
	if _, open := surfaceOf[*uisurfaces.Card](m); !open {
		t.Fatalf("f2 opened nothing; err %q", m.errBar.text)
	}
	m.handleKey(key("esc"))
	if _, open := surfaceOf[*uisurfaces.Card](m); open || !m.focused() {
		t.Fatalf("esc left card open %v, focused %v", open, m.focused())
	}
}

func TestClickingTheFocusedMarkOpensTheCard(t *testing.T) {
	m := focusAsking(t)
	preparedView(m)
	x, y := paintedCell(t, m, "open what it asks")
	m = leftClick(m, x, y)
	if _, open := surfaceOf[*uisurfaces.Card](m); !open {
		t.Fatalf("the click opened nothing; err %q", m.errBar.text)
	}
}

func TestFocusedQuietSessionHasNoMark(t *testing.T) {
	m, _ := askingModel(t)
	m.selectSessionRow(t, "beta")
	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	*m = *updated.(*Model)
	m.drainEffects(t)
	m.prepareFrame()
	if footer := ansi.Strip(m.viewFooter()); strings.Contains(footer, "open what it asks") {
		t.Fatalf("footer = %q", footer)
	}
}
