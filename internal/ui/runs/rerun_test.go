package runs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRequestRerunWritesExactlyTheContractOnce(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.FixedZone("x", -3*3600))
	wrote, err := RequestRerun(dir, "run1", now)
	if err != nil || !wrote {
		t.Fatalf("first request = %v, %v; want written", wrote, err)
	}
	data, _ := os.ReadFile(RerunPath(dir, "run1"))
	if got, want := string(data), `{"version":1,"requestedAt":"2026-11-25T15:00:00Z"}`+"\n"; got != want {
		t.Fatalf("file = %q, want %q", got, want)
	}
	wrote, err = RequestRerun(dir, "run1", now.Add(time.Hour))
	if err != nil || wrote {
		t.Fatalf("second request = %v, %v; want a no-op", wrote, err)
	}
	again, _ := os.ReadFile(RerunPath(dir, "run1"))
	if string(again) != string(data) {
		t.Fatalf("second press changed the file: %q", again)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("dir holds %d files, want only the request (no temp files)", len(entries))
	}
}

func TestRequestRerunRefusesABadRunID(t *testing.T) {
	if _, err := RequestRerun(t.TempDir(), "a/b", time.Now()); err == nil {
		t.Fatal("a path separator in the id was accepted")
	}
	if _, err := RequestRerun(t.TempDir(), "", time.Now()); err == nil {
		t.Fatal("an empty id was accepted")
	}
}

func TestRequestedListsRequestFilesAndLoadIgnoresThem(t *testing.T) {
	dir := t.TempDir()
	writeRun(t, dir, "a.json", record("a", "failed", "2026-01-02T00:00:00Z", ""))
	if _, err := RequestRerun(dir, "a", time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := Requested(dir); len(got) != 1 || !got["a"] {
		t.Fatalf("Requested = %v, want only a", got)
	}
	if got, _ := Load(dir); len(got) != 1 {
		t.Fatalf("Load = %d records, want 1", len(got))
	}
	if got := Requested(filepath.Join(dir, "absent")); len(got) != 0 {
		t.Fatalf("Requested of a missing dir = %v", got)
	}
}
