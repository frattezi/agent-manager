package rail_test

import (
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/keybind"
	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	"github.com/YoanWai/agent-manager/internal/ui/rail"
)

func babysitFrame(t *testing.T, mode presentation.GlyphMode, sessions ...rail.Session) rail.Frame {
	t.Helper()
	model := rail.New(nil)
	model.Reconcile(rail.Snapshot{Sessions: sessions})
	ctx := renderContext(60, 12)
	ctx.GlyphMode = mode
	return model.Render(ctx, rail.Frame{})
}

func TestABabysittingRowWearsTheBottleInPlaceOfItsStatus(t *testing.T) {
	now := time.Now()
	frame := babysitFrame(t, presentation.GlyphModeAdvanced,
		rail.Session{ID: "sitter", Name: "sitter", Status: "idle", Babysitting: true, CreatedAt: now},
		rail.Session{ID: "plain", Name: "plain", Status: "idle", CreatedAt: now.Add(-time.Second)},
	)
	sitter := paintedRow(t, frame, "sitter")
	if !strings.Contains(sitter, "🍼 ⠿ sitter") || strings.Contains(sitter, "○") {
		t.Fatalf("babysitting row = %q, want the bottle where the status glyph was, a gap before the grip", sitter)
	}
}

func TestTheBottleIsAPlainLetterInBasicGlyphs(t *testing.T) {
	frame := babysitFrame(t, presentation.GlyphModeBasic,
		rail.Session{ID: "sitter", Name: "sitter", Status: "working", Babysitting: true, CreatedAt: time.Now()})
	if row := paintedRow(t, frame, "sitter"); !strings.Contains(row, "b = sitter") {
		t.Fatalf("basic row = %q, want b in the status column", row)
	}
}

func TestAStatusThatNeedsTheOwnerOutranksTheBottle(t *testing.T) {
	for _, status := range []string{"waiting", "errored", "dead"} {
		frame := babysitFrame(t, presentation.GlyphModeAdvanced,
			rail.Session{ID: "sitter", Name: "sitter", Status: status, Babysitting: true, CreatedAt: time.Now()})
		if row := paintedRow(t, frame, "sitter"); strings.Contains(row, "🍼") {
			t.Fatalf("%s row = %q hides its status behind the bottle", status, row)
		}
	}
}

func TestTheRowLegendNamesTheBottleOnlyWhileBabysitting(t *testing.T) {
	model := rail.New(nil)
	ctx := rail.LegendContext{ListKeys: keybind.DefaultList(), GlyphMode: presentation.GlyphModeAdvanced, RowOK: true,
		Row: rail.Selection{Kind: rail.SessionRow, SessionID: "sitter"}, Session: rail.Session{ID: "sitter", Status: "idle", Babysitting: true}}
	if !legendNames(model.RowLegend(ctx), "🍼", "babysitting Tickets to merged") {
		t.Fatalf("legend = %+v", model.RowLegend(ctx))
	}
	ctx.Session.Babysitting = false
	if legendNames(model.RowLegend(ctx), "🍼", "babysitting Tickets to merged") {
		t.Fatal("the legend names a babysit the session is not running")
	}
}
