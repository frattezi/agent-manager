package rebuild

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeBinary(t *testing.T, path, content string, installed time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, installed, installed); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveCopiesTheBinaryUnderItsStampWithItsInstallTime(t *testing.T) {
	versions := filepath.Join(t.TempDir(), "versions")
	target := filepath.Join(t.TempDir(), "manager")
	installed := time.Date(2026, 11, 1, 9, 30, 0, 0, time.UTC)
	writeBinary(t, target, "old build", installed)

	entry, err := Archive(versions, target, "0.40.0-3-gabc1234")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Dir != filepath.Join(versions, "0.40.0-3-gabc1234") {
		t.Fatalf("archived in %s", entry.Dir)
	}
	data, err := os.ReadFile(filepath.Join(entry.Dir, "manager"))
	if err != nil || string(data) != "old build" {
		t.Fatalf("archived binary reads %q, %v", data, err)
	}
	entries, err := Entries(versions)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries %v, %v", entries, err)
	}
	got := entries[0]
	if got.Version != "0.40.0-3-gabc1234" || got.Binary != "manager" || !got.InstalledAt.Equal(installed) {
		t.Fatalf("manifest reads %+v", got)
	}
}

func TestArchiveKeepsAStampToOnePathElement(t *testing.T) {
	versions := t.TempDir()
	target := filepath.Join(t.TempDir(), "manager")
	writeBinary(t, target, "x", time.Now())
	entry, err := Archive(versions, target, "../evil/..")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(entry.Dir) != versions {
		t.Fatalf("archived outside the store: %s", entry.Dir)
	}
}

func TestPruneKeepsTheMostRecentlyInstalledBuilds(t *testing.T) {
	versions := t.TempDir()
	target := filepath.Join(t.TempDir(), "manager")
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for i, stamp := range []string{"a", "b", "c", "d", "e"} {
		writeBinary(t, target, stamp, base.Add(time.Duration(i)*time.Hour))
		if _, err := Archive(versions, target, stamp); err != nil {
			t.Fatal(err)
		}
	}
	foreign := filepath.Join(versions, "not-ours")
	if err := os.Mkdir(foreign, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Prune(versions, Kept); err != nil {
		t.Fatal(err)
	}
	entries, err := Entries(versions)
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, entry := range entries {
		kept = append(kept, entry.Version)
	}
	if len(kept) != 3 || kept[0] != "e" || kept[1] != "d" || kept[2] != "c" {
		t.Fatalf("kept %v, want [e d c]", kept)
	}
	for _, gone := range []string{"a", "b"} {
		if _, err := os.Stat(filepath.Join(versions, gone)); !os.IsNotExist(err) {
			t.Fatalf("%s was not pruned: %v", gone, err)
		}
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("a directory the store did not write was removed: %v", err)
	}
}

func TestArchivingAStampAgainReplacesItsCopy(t *testing.T) {
	versions := t.TempDir()
	target := filepath.Join(t.TempDir(), "manager")
	writeBinary(t, target, "first", time.Now().Add(-time.Hour))
	if _, err := Archive(versions, target, "dev"); err != nil {
		t.Fatal(err)
	}
	writeBinary(t, target, "second", time.Now())
	entry, err := Archive(versions, target, "dev")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(entry.Dir, "manager"))
	if string(data) != "second" {
		t.Fatalf("archive holds %q", data)
	}
	if entries, _ := Entries(versions); len(entries) != 1 {
		t.Fatalf("one stamp archived as %d builds", len(entries))
	}
}
