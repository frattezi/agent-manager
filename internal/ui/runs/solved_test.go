package runs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMarkSolvedWritesTheMarkOnceAndUnmarkRemovesIt(t *testing.T) {
	dir := t.TempDir()
	writeRun(t, dir, "a.json", record("a", "failed", "2026-01-02T00:00:00Z", ""))
	before, _ := os.ReadFile(filepath.Join(dir, "a.json"))
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.FixedZone("x", -3*3600))
	wrote, err := MarkSolved(dir, "a", now)
	if err != nil || !wrote {
		t.Fatalf("mark = %v, %v; want written", wrote, err)
	}
	data, _ := os.ReadFile(SolvedPath(dir, "a"))
	if got, want := string(data), `{"version":1,"solvedAt":"2026-11-25T15:00:00Z"}`+"\n"; got != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
	if wrote, err := MarkSolved(dir, "a", now.Add(time.Hour)); err != nil || wrote {
		t.Fatalf("second mark = %v, %v; want a no-op", wrote, err)
	}
	if got := Solved(dir); len(got) != 1 || !got["a"] {
		t.Fatalf("Solved = %v, want only a", got)
	}
	if got, _ := Load(dir); len(got) != 1 {
		t.Fatalf("Load = %d records, want the mark skipped", len(got))
	}
	after, _ := os.ReadFile(filepath.Join(dir, "a.json"))
	if string(after) != string(before) {
		t.Fatal("marking a Ticket solved changed the Run record")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatalf("dir holds %d files, want the record and the mark (no temp files)", len(entries))
	}
	if removed, err := UnmarkSolved(dir, "a"); err != nil || !removed {
		t.Fatalf("unmark = %v, %v; want removed", removed, err)
	}
	if removed, err := UnmarkSolved(dir, "a"); err != nil || removed {
		t.Fatalf("second unmark = %v, %v; want a no-op", removed, err)
	}
	if got := Solved(dir); len(got) != 0 {
		t.Fatalf("Solved after unmark = %v", got)
	}
}

func TestSolvedMarkRefusesABadRunID(t *testing.T) {
	for _, id := range []string{"", "a/b"} {
		if _, err := MarkSolved(t.TempDir(), id, time.Now()); err == nil {
			t.Fatalf("mark accepted %q", id)
		}
		if _, err := UnmarkSolved(t.TempDir(), id); err == nil {
			t.Fatalf("unmark accepted %q", id)
		}
	}
}
