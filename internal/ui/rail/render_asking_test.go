package rail_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/keybind"
	"github.com/YoanWai/agent-manager/internal/ui/rail"
	"github.com/charmbracelet/x/ansi"
)

func askingSnapshot() rail.Snapshot {
	return rail.Snapshot{Sessions: []rail.Session{
		{ID: "asks", Name: "asks", Status: "idle", Asking: true, AskingGlyph: "ASK", CreatedAt: time.Now()},
		{ID: "quiet", Name: "quiet", Status: "idle", AskingGlyph: "ASK", CreatedAt: time.Now()},
	}}
}

func paintedRow(t *testing.T, frame rail.Frame, id string) string {
	t.Helper()
	for _, line := range frame.Lines {
		if line.HitOK && line.Hit.SessionID == id {
			return ansi.Strip(line.Text)
		}
	}
	t.Fatalf("%s is not painted", id)
	return ""
}

func TestAnAskingSessionsRowCarriesTheMark(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(askingSnapshot())
	frame := model.Render(renderContext(60, 10), rail.Frame{})
	if row := paintedRow(t, frame, "asks"); !strings.Contains(row, "asks ASK") {
		t.Fatalf("asking row = %q, want the mark after the name", row)
	}
	if row := paintedRow(t, frame, "quiet"); strings.Contains(row, "ASK") {
		t.Fatalf("a session asking nothing is marked: %q", row)
	}
}

func TestTheAttentionFilterKeepsAnAskingSession(t *testing.T) {
	model := rail.New(nil)
	snapshot := askingSnapshot()
	snapshot.Sessions = append(snapshot.Sessions, rail.Session{ID: "done", Name: "done", Status: "finished"})
	model.Reconcile(snapshot)
	model.SetFilteringAttention(true)
	model.Reconcile(snapshot)
	ids := model.ListedSessionIDs()
	if !slices.Contains(ids, "asks") || !slices.Contains(ids, "done") {
		t.Fatalf("filtered = %v, want the asking and the finished session", ids)
	}
	for _, id := range ids {
		if id == "quiet" && !selectedIs(model, "quiet") {
			t.Fatalf("filtered = %v keeps an idle session asking nothing", ids)
		}
	}
}

func selectedIs(model rail.Model, id string) bool {
	selected, ok := model.SelectedSession()
	return ok && selected == id
}

func TestTheSurfaceKeyAsksToOpenWhatTheSessionAsks(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(askingSnapshot())
	model.FocusSession("asks")
	keys := keybind.DefaultList()
	decision := model.Key(keyFor(keys.Binding(keybind.AgentSurface).Keys()[0].Tea()), rail.Frame{}, rail.KeyContext{ListKeys: keys})
	if decision.Intent.Kind != rail.OpenSurface || decision.Intent.Target.SessionID != "asks" {
		t.Fatalf("decision = %+v", decision)
	}
}

func TestTheRowLegendNamesTheSurfaceKeyOnlyWhileAsking(t *testing.T) {
	model := rail.New(nil)
	keys := keybind.DefaultList()
	ctx := rail.LegendContext{ListKeys: keys, RowOK: true, Row: rail.Selection{Kind: rail.SessionRow, SessionID: "asks"}, Session: rail.Session{ID: "asks", Status: "idle", Asking: true}}
	if !legendNames(model.RowLegend(ctx), "i", "open what it asks") {
		t.Fatalf("legend = %+v", model.RowLegend(ctx))
	}
	ctx.Session.Asking = false
	if legendNames(model.RowLegend(ctx), "i", "open what it asks") {
		t.Fatal("the legend offers a surface to a session asking nothing")
	}
}

func legendNames(legend rail.Legend, key, does string) bool {
	for _, pair := range legend.Pairs {
		if pair[0] == key && pair[1] == does {
			return true
		}
	}
	return false
}
