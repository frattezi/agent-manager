package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/rebuild"
)

// archiveForRollback fills configDir's versions store with one build per
// stamp, oldest first, and returns the target a rollback replaces.
func archiveForRollback(t *testing.T, configDir string, stamps ...string) string {
	t.Helper()
	source := filepath.Join(t.TempDir(), "manager")
	base := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	for i, stamp := range stamps {
		if err := os.WriteFile(source, []byte("build "+stamp), 0o755); err != nil {
			t.Fatal(err)
		}
		installed := base.Add(time.Duration(i) * time.Hour)
		if err := os.Chtimes(source, installed, installed); err != nil {
			t.Fatal(err)
		}
		if _, err := rebuild.Archive(rebuild.VersionsDir(configDir), source, stamp); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(t.TempDir(), "my-launcher")
	if err := os.WriteFile(target, []byte("running"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := rebuildExecutable
	t.Cleanup(func() { rebuildExecutable = old })
	rebuildExecutable = func() (string, error) { return target, nil }
	return target
}

func TestRollbackListPrintsTheArchiveNewestFirstMarkingTheInstalledBuild(t *testing.T) {
	configDir := t.TempDir()
	target := archiveForRollback(t, configDir, "1.0.0-1-gaaa", "1.0.0-2-gbbb", "1.0.0-3-gccc")
	var out bytes.Buffer
	if err := runRollback(&out, "1.0.0-2-gbbb", "am", []string{"--list"}, configDir); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("listed:\n%s", out.String())
	}
	for i, want := range []string{"  1.0.0-3-gccc", "* 1.0.0-2-gbbb", "  1.0.0-1-gaaa"} {
		if !strings.HasPrefix(lines[i], want) {
			t.Fatalf("line %d is %q, want %q first:\n%s", i, lines[i], want, out.String())
		}
	}
	installed := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC).Local().Format(time.DateTime)
	if !strings.Contains(lines[1], installed) {
		t.Fatalf("line %q does not carry the install time %s", lines[1], installed)
	}
	if data, _ := os.ReadFile(target); string(data) != "running" {
		t.Fatalf("--list touched the target: %q", data)
	}
}

func TestRollbackDefaultsToTheNewestBuildThatIsNotInstalled(t *testing.T) {
	configDir := t.TempDir()
	target := archiveForRollback(t, configDir, "1.0.0-1-gaaa", "1.0.0-2-gbbb")
	var out bytes.Buffer
	if err := runRollback(&out, "1.0.0-3-gccc", "am", nil, configDir); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(target); string(data) != "build 1.0.0-2-gbbb" {
		t.Fatalf("target holds %q", data)
	}
	for _, want := range []string{"1.0.0-2-gbbb", target, "restart"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output does not say %q:\n%s", want, out.String())
		}
	}
	if entries, _ := rebuild.Entries(rebuild.VersionsDir(configDir)); len(entries) != 2 {
		t.Fatalf("the archive changed: %v", entries)
	}
}

func TestRollbackSkipsTheInstalledBuildByDefault(t *testing.T) {
	configDir := t.TempDir()
	target := archiveForRollback(t, configDir, "1.0.0-1-gaaa", "1.0.0-2-gbbb")
	if err := runRollback(&bytes.Buffer{}, "1.0.0-2-gbbb", "am", nil, configDir); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(target); string(data) != "build 1.0.0-1-gaaa" {
		t.Fatalf("target holds %q", data)
	}
}

func TestRollbackInstallsTheNamedBuild(t *testing.T) {
	configDir := t.TempDir()
	target := archiveForRollback(t, configDir, "1.0.0-1-gaaa", "1.0.0-2-gbbb", "1.0.0-3-gccc")
	if err := runRollback(&bytes.Buffer{}, "dev", "am", []string{"1.0.0-1-gaaa"}, configDir); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(target); string(data) != "build 1.0.0-1-gaaa" {
		t.Fatalf("target holds %q", data)
	}
}

func TestRollbackToAnUnknownNameFailsWithTheArchive(t *testing.T) {
	configDir := t.TempDir()
	target := archiveForRollback(t, configDir, "1.0.0-1-gaaa", "1.0.0-2-gbbb")
	err := runRollback(&bytes.Buffer{}, "dev", "am", []string{"9.9.9"}, configDir)
	if err == nil {
		t.Fatal("an unknown name was rolled back to")
	}
	for _, want := range []string{"9.9.9", "1.0.0-1-gaaa", "1.0.0-2-gbbb"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not name %q", err, want)
		}
	}
	if data, _ := os.ReadFile(target); string(data) != "running" {
		t.Fatalf("target holds %q", data)
	}
}

func TestRollbackWithAnEmptyArchiveNamesRebuild(t *testing.T) {
	for _, args := range [][]string{nil, {"--list"}, {"1.0.0"}} {
		err := runRollback(&bytes.Buffer{}, "dev", "my-launcher", args, t.TempDir())
		if err == nil || !strings.Contains(err.Error(), "my-launcher rebuild") {
			t.Fatalf("rollback %v on an empty archive: %v", args, err)
		}
	}
}

func TestRollbackRefusesAListWithAVersion(t *testing.T) {
	configDir := t.TempDir()
	archiveForRollback(t, configDir, "1.0.0-1-gaaa")
	if err := runRollback(&bytes.Buffer{}, "dev", "am", []string{"--list", "1.0.0-1-gaaa"}, configDir); err == nil {
		t.Fatal("--list took a version")
	}
}
