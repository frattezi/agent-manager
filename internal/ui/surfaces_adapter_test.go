package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/keybind"
	"github.com/YoanWai/agent-manager/internal/store"
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	uisurfaces "github.com/YoanWai/agent-manager/internal/ui/surfaces"
	"github.com/charmbracelet/x/ansi"
)

func roundDocument(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("a2ui", "testdata", "round.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// publishTestSurface stores a surface for the named session the way
// agent-manager surface publish does, and lets the poll read it.
func publishTestSurface(t *testing.T, m *Model, name, id, document string, answered bool) string {
	t.Helper()
	m.selectSessionRow(t, name)
	sess := railSelectedSession(m)
	surface := store.AgentSurface{ID: id, SessionID: sess.ID, PublishedAt: time.Now(), Document: document}
	if answered {
		surface.AnsweredAt, surface.Answer = time.Now(), "done"
	}
	if err := m.services.store.PublishAgentSurface(surface, 50); err != nil {
		t.Fatal(err)
	}
	m.applyTestMsg(t, surfacesReadNow())
	m.drainEffects(t)
	return sess.ID
}

func askingModel(t *testing.T) (*Model, string) {
	t.Helper()
	m := buildModel(t)
	m.layout.width, m.layout.height = 120, 40
	createSession(t, m, "alpha", t.TempDir(), "")
	createSession(t, m, "beta", t.TempDir(), "")
	id := publishTestSurface(t, m, "alpha", "round-1", roundDocument(t), false)
	return m, id
}

// railRowText is the session's row as the rail paints it.
func railRowText(t *testing.T, m *Model, name string) string {
	t.Helper()
	for _, line := range m.entryLines(nil, 0, 60, 30) {
		if line.HitOK && line.Hit.Kind == uirail.SessionRow {
			if sess, ok := m.sessionByID(line.Hit.SessionID); ok && sess.Name == name {
				return ansi.Strip(line.Text)
			}
		}
	}
	t.Fatalf("%s has no row", name)
	return ""
}

func TestAStoredSurfaceMarksItsSessionsRowAndNoOther(t *testing.T) {
	m, id := askingModel(t)
	if !m.workspace.surfaces.inbox.Asking(id) {
		t.Fatal("the read did not find the stored surface")
	}
	mark := m.glyphs().AskingGlyph
	if row := railRowText(t, m, "alpha"); !strings.Contains(row, "alpha "+mark) {
		t.Fatalf("alpha's row = %q, want the %q mark", row, mark)
	}
	if row := railRowText(t, m, "beta"); strings.Contains(row, "beta "+mark) {
		t.Fatalf("beta's row is marked: %q", row)
	}
}

func TestAnAnsweredSurfaceLeavesNoMark(t *testing.T) {
	m := buildModel(t)
	m.layout.width, m.layout.height = 120, 40
	createSession(t, m, "alpha", t.TempDir(), "")
	id := publishTestSurface(t, m, "alpha", "round-1", roundDocument(t), true)
	if m.workspace.surfaces.inbox.Asking(id) {
		t.Fatal("an answered surface still marks its session")
	}
	if row := railRowText(t, m, "alpha"); strings.Contains(row, "alpha "+m.glyphs().AskingGlyph) {
		t.Fatalf("alpha's row = %q", row)
	}
}

func TestTheAttentionFilterKeepsAnAskingSession(t *testing.T) {
	m, id := askingModel(t)
	m.selectSessionRow(t, "beta")
	m.handleKey(key("w"))
	m.rebuildRows()
	var listed []string
	for _, row := range m.rail.Rows() {
		if row.Kind == uirail.SessionRow {
			listed = append(listed, row.SessionID)
		}
	}
	found := false
	for _, sid := range listed {
		found = found || sid == id
	}
	if !m.rail.FilteringAttention() || !found {
		t.Fatalf("filtering %v, listed %v; want alpha kept", m.rail.FilteringAttention(), listed)
	}
}

func TestNoSurfaceOpensOnItsOwn(t *testing.T) {
	m, _ := askingModel(t)
	for range 3 {
		m.applyTestMsg(t, surfacesReadNow())
		m.drainEffects(t)
	}
	if _, open := surfaceOf[*uisurfaces.Card](m); open || len(m.surfaces) != 0 {
		t.Fatalf("a read opened %d surfaces", len(m.surfaces))
	}
}

func TestTheKeyOpensTheSurfaceAndEscLeavesItUnansweredWithTheMark(t *testing.T) {
	m, id := askingModel(t)
	m.selectSessionRow(t, "alpha")
	m.handleKey(key(m.listGlyph(keybind.AgentSurface)))
	card, open := surfaceOf[*uisurfaces.Card](m)
	if !open || card.SessionID() != id || card.SurfaceID() != "round-1" {
		t.Fatalf("open %v, stack %d, err %q", open, len(m.surfaces), m.errBar.text)
	}
	view := ansi.Strip(preparedView(m))
	for _, want := range []string{"alpha is asking", "1. Which package owns the A2UI format?", "(*) A named cap", "esc"} {
		if !strings.Contains(view, want) {
			t.Fatalf("the card is missing %q:\n%s", want, view)
		}
	}
	// The keys beneath the card are not reachable through it: d would
	// delete the session, and here it types nothing and opens no confirm.
	m.handleKey(key("d"))
	if top, _ := m.top(); top != card {
		t.Fatalf("d reached the list: top is %T", top)
	}
	m.handleKey(key("esc"))
	if _, open := surfaceOf[*uisurfaces.Card](m); open {
		t.Fatal("esc left the card open")
	}
	surfaces, err := m.services.store.AgentSurfaces(id)
	if err != nil || len(surfaces) != 1 || !surfaces[0].AnsweredAt.IsZero() {
		t.Fatalf("after esc the surface is %+v, %v", surfaces, err)
	}
	m.applyTestMsg(t, surfacesReadNow())
	m.drainEffects(t)
	if row := railRowText(t, m, "alpha"); !strings.Contains(row, "alpha "+m.glyphs().AskingGlyph) {
		t.Fatalf("the mark went with the card: %q", row)
	}
}

func TestTheFooterNamesTheKeyOnAnAskingRow(t *testing.T) {
	m, _ := askingModel(t)
	m.selectSessionRow(t, "alpha")
	if footer := ansi.Strip(m.viewFooter()); !strings.Contains(footer, "open what it asks") {
		t.Fatalf("footer = %q", footer)
	}
	m.selectSessionRow(t, "beta")
	if footer := ansi.Strip(m.viewFooter()); strings.Contains(footer, "open what it asks") {
		t.Fatalf("a quiet row offers a surface: %q", footer)
	}
}

func TestTheKeyOnASessionAskingNothingSaysSo(t *testing.T) {
	m, _ := askingModel(t)
	m.selectSessionRow(t, "beta")
	m.handleKey(key(m.listGlyph(keybind.AgentSurface)))
	if len(m.surfaces) != 0 || !strings.Contains(m.errBar.text, "is not asking anything") {
		t.Fatalf("stack %d, err %q", len(m.surfaces), m.errBar.text)
	}
}

// Clicking the key on the footer opens the card the way typing it does.
func TestClickingTheFooterKeyOpensTheSurface(t *testing.T) {
	m, _ := askingModel(t)
	m.selectSessionRow(t, "alpha")
	preparedView(m)
	x, y := paintedCell(t, m, "open what it asks")
	m = leftClick(m, x, y)
	if _, open := surfaceOf[*uisurfaces.Card](m); !open {
		t.Fatalf("the click opened nothing; err %q", m.errBar.text)
	}
}
