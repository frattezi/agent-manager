package rebuild

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func archiveBuilds(t *testing.T, versions string, stamps ...string) {
	t.Helper()
	target := filepath.Join(t.TempDir(), "manager")
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for i, stamp := range stamps {
		writeBinary(t, target, "build "+stamp, base.Add(time.Duration(i)*time.Hour))
		if _, err := Archive(versions, target, stamp); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPickDefaultsToTheNewestBuildThatIsNotInstalled(t *testing.T) {
	versions := t.TempDir()
	archiveBuilds(t, versions, "a", "b", "c")
	entries, _ := Entries(versions)
	if got, err := Pick(entries, "z", ""); err != nil || got.Version != "c" {
		t.Fatalf("picked %+v, %v; want c", got, err)
	}
	if got, err := Pick(entries, "c", ""); err != nil || got.Version != "b" {
		t.Fatalf("picked %+v, %v; want b past the installed c", got, err)
	}
}

func TestPickNamesABuildOrSaysItIsUnknown(t *testing.T) {
	versions := t.TempDir()
	archiveBuilds(t, versions, "a", "b")
	entries, _ := Entries(versions)
	if got, err := Pick(entries, "b", "a"); err != nil || got.Version != "a" {
		t.Fatalf("picked %+v, %v; want a", got, err)
	}
	if _, err := Pick(entries, "b", "nope"); !errors.Is(err, ErrUnknownBuild) {
		t.Fatalf("an unknown name gave %v", err)
	}
}

func TestPickRefusesAnEmptyArchiveAndOneHoldingOnlyTheInstalledBuild(t *testing.T) {
	if _, err := Pick(nil, "a", ""); !errors.Is(err, ErrEmptyArchive) {
		t.Fatalf("an empty archive gave %v", err)
	}
	versions := t.TempDir()
	archiveBuilds(t, versions, "a")
	entries, _ := Entries(versions)
	if _, err := Pick(entries, "a", ""); !errors.Is(err, ErrNoOtherBuild) {
		t.Fatalf("an archive of only the installed build gave %v", err)
	}
}

func TestRestoreCopiesTheBuildOverTheTargetAndLeavesTheArchive(t *testing.T) {
	versions := t.TempDir()
	archiveBuilds(t, versions, "a", "b")
	entries, _ := Entries(versions)
	target := filepath.Join(t.TempDir(), "launcher")
	writeBinary(t, target, "current", time.Now())
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	installed, err := Restore(entries[1], link)
	if err != nil {
		t.Fatal(err)
	}
	if installed != target {
		t.Fatalf("installed at %s, want %s", installed, target)
	}
	if data, _ := os.ReadFile(target); string(data) != "build a" {
		t.Fatalf("target holds %q", data)
	}
	if info, _ := os.Stat(target); info.Mode().Perm() != 0o755 {
		t.Fatalf("target mode %v", info.Mode())
	}
	after, _ := Entries(versions)
	if len(after) != 2 {
		t.Fatalf("the archive changed: %v", after)
	}
	if data, _ := os.ReadFile(filepath.Join(entries[1].Dir, entries[1].Binary)); string(data) != "build a" {
		t.Fatalf("the archived copy reads %q", data)
	}
}
