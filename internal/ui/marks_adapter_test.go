package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/store"
	"github.com/YoanWai/agent-manager/internal/ui/presentation"
)

// A live wrapper's mark shows on its row; one whose process is gone does not,
// whatever the store still holds.
func TestTheBabysitMarkFollowsTheProcessThatSetIt(t *testing.T) {
	m := buildModel(t)
	m.layout.width, m.layout.height = 120, 40
	m.prefs.glyphMode = presentation.GlyphModeAdvanced
	createSession(t, m, "sitter", t.TempDir(), "")
	createSession(t, m, "orphan", t.TempDir(), "")
	for name, pid := range map[string]int{"sitter": os.Getpid(), "orphan": 1 << 30} {
		m.selectSessionRow(t, name)
		mark := store.SessionMark{SessionID: railSelectedSession(m).ID, Name: store.MarkBabysit, Pid: pid, SetAt: time.Now()}
		if err := m.services.store.SetSessionMark(mark); err != nil {
			t.Fatal(err)
		}
	}
	m.applyTestMsg(t, marksReadNow())
	m.drainEffects(t)
	if row := railRowText(t, m, "sitter"); !strings.Contains(row, "🍼") {
		t.Fatalf("sitter row = %q, want the bottle", row)
	}
	if row := railRowText(t, m, "orphan"); strings.Contains(row, "🍼") {
		t.Fatalf("orphan row = %q wears a mark whose process is gone", row)
	}
}
