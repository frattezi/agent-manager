package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func publishSurface(t *testing.T, st *Store, id, sessionID string, at time.Time, keep int) {
	t.Helper()
	if err := st.PublishAgentSurface(AgentSurface{ID: id, SessionID: sessionID, PublishedAt: at, Document: `[]`}, keep); err != nil {
		t.Fatalf("PublishAgentSurface %s: %v", id, err)
	}
}

func surfaceIDs(t *testing.T, st *Store, sessionID string) []string {
	t.Helper()
	surfaces, err := st.AgentSurfaces(sessionID)
	if err != nil {
		t.Fatalf("AgentSurfaces: %v", err)
	}
	ids := make([]string, 0, len(surfaces))
	for _, s := range surfaces {
		ids = append(ids, s.ID)
	}
	return ids
}

func TestAPublishedSurfaceReadsBackAsWritten(t *testing.T) {
	st := newTestStore(t)
	at := time.Unix(0, 1_700_000_000_000_000_000)
	doc := `[{"version":"v0.9"}]`
	if err := st.PublishAgentSurface(AgentSurface{ID: "s1", SessionID: "a1", PublishedAt: at, Document: doc}, 10); err != nil {
		t.Fatal(err)
	}
	got, err := st.AgentSurface("s1")
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionID != "a1" || got.Document != doc || !got.PublishedAt.Equal(at) || !got.AnsweredAt.IsZero() || got.Answer != "" {
		t.Fatalf("surface = %+v", got)
	}
	if _, err := st.AgentSurface("nope"); !errors.Is(err, ErrAgentSurfaceNotFound) {
		t.Fatalf("missing surface err = %v", err)
	}
}

// The cap counts per session, so a chatty session cannot trim another's
// history, and the oldest go first.
func TestPublishingPastTheCapDropsTheOldest(t *testing.T) {
	st := newTestStore(t)
	base := time.Unix(1_700_000_000, 0)
	publishSurface(t, st, "other", "b2", base, 2)
	for i, id := range []string{"r1", "r2", "r3", "r4"} {
		publishSurface(t, st, id, "a1", base.Add(time.Duration(i)*time.Second), 2)
	}
	if got := surfaceIDs(t, st, "a1"); len(got) != 2 || got[0] != "r3" || got[1] != "r4" {
		t.Fatalf("kept = %v, want [r3 r4]", got)
	}
	if got := surfaceIDs(t, st, "b2"); len(got) != 1 {
		t.Fatalf("another session's surfaces = %v", got)
	}
}

func TestDeletingASessionTakesItsSurfaces(t *testing.T) {
	st := newTestStore(t)
	for _, id := range []string{"a1", "b2"} {
		if err := st.CreateSession(sample(id, "")); err != nil {
			t.Fatal(err)
		}
	}
	publishSurface(t, st, "s1", "a1", time.Now(), 10)
	publishSurface(t, st, "s2", "b2", time.Now(), 10)
	if err := st.Delete("a1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := surfaceIDs(t, st, "a1"); len(got) != 0 {
		t.Fatalf("deleted session kept %v", got)
	}
	if got := surfaceIDs(t, st, "b2"); len(got) != 1 {
		t.Fatalf("another session lost its surfaces: %v", got)
	}
}

func TestDeletingANestedSessionTakesItsSurfaces(t *testing.T) {
	st := newTestStore(t)
	parent, child := sample("p1", ""), sample("c1", "")
	child.ParentID = parent.ID
	for _, sess := range []Session{parent, child} {
		if err := st.CreateSession(sess); err != nil {
			t.Fatal(err)
		}
	}
	publishSurface(t, st, "s1", child.ID, time.Now(), 10)
	if err := st.DeleteChild(child.ID, parent.ID, func() error { return nil }); err != nil {
		t.Fatalf("DeleteChild: %v", err)
	}
	if got := surfaceIDs(t, st, child.ID); len(got) != 0 {
		t.Fatalf("deleted child kept %v", got)
	}
}

// Every Open runs the migrations again; a database that already has the
// table keeps its rows.
func TestSurfacesSurviveAReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	publishSurface(t, st, "s1", "a1", time.Now(), 10)
	st.Close()
	if st, err = Open(path); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if got := surfaceIDs(t, st, "a1"); len(got) != 1 {
		t.Fatalf("after reopen = %v", got)
	}
}

// A database an earlier release wrote has no table; opening it adds one.
func TestAnUpgradedDatabaseGetsTheSurfacesTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(`CREATE TABLE sessions (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, tool TEXT NOT NULL, cwd TEXT NOT NULL,
		group_name TEXT NOT NULL, status TEXT NOT NULL, archived INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL, last_status_at INTEGER NOT NULL
	)`); err != nil {
		t.Fatal(err)
	}
	legacy.Close()
	st, err := Open(path)
	if err != nil {
		t.Fatalf("open upgraded: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	publishSurface(t, st, "s1", "a1", time.Now(), 10)
	if got := surfaceIDs(t, st, "a1"); len(got) != 1 {
		t.Fatalf("upgraded = %v", got)
	}
}
