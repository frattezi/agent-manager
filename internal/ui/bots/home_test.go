package bots

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func readMarker(t *testing.T, dir string) marker {
	t.Helper()
	var got marker
	if _, err := toml.DecodeFile(filepath.Join(dir, MemoryMarker), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func repoFixture(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestOwnDirIsTheProfileNamedUnderTheRoot(t *testing.T) {
	if got := OwnDir("/r", Profile{ID: "b1", Name: "Fractory Operator"}); got != filepath.Join("/r", "fractory-operator") {
		t.Fatalf("own dir = %q", got)
	}
}

func TestPrepareCreatesTheOwnDirAndMarksItsMemory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Work", "Bots", "operator")
	got, err := PrepareRequest{Profile: Profile{Name: "Operator", MemoryProject: "ops"}, Dir: dir, Owned: true}.Run(Services{})
	if err != nil {
		t.Fatal(err)
	}
	if got.(Prepared).Dir != dir {
		t.Fatalf("prepared = %+v; want %s", got, dir)
	}
	if m := readMarker(t, dir); m != (marker{Workspace: MemoryWorkspace, Project: "ops"}) {
		t.Fatalf("marker = %+v", m)
	}
}

func TestPrepareMarksADirectoryThatIsNoRepository(t *testing.T) {
	dir := t.TempDir()
	if _, err := (PrepareRequest{Profile: Profile{MemoryProject: "ops"}, Dir: dir}).Run(Services{}); err != nil {
		t.Fatal(err)
	}
	if m := readMarker(t, dir); m.Project != "ops" || m.Workspace == "" {
		t.Fatalf("marker = %+v; want both keys", m)
	}
}

func TestPrepareWritesNoMarkerIntoARepository(t *testing.T) {
	repo := repoFixture(t)
	inside := filepath.Join(repo, "cmd")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{repo, inside} {
		if _, err := (PrepareRequest{Profile: Profile{MemoryProject: "ops"}, Dir: dir}).Run(Services{}); err != nil {
			t.Fatal(err)
		}
	}
	err := filepath.WalkDir(repo, func(path string, _ os.DirEntry, err error) error {
		if err == nil && filepath.Base(path) == MemoryMarker {
			t.Errorf("a marker was written into the repository at %s", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPrepareWithNoMemoryWritesNoMarker(t *testing.T) {
	dir := t.TempDir()
	if _, err := (PrepareRequest{Profile: Profile{Name: "Operator"}, Dir: dir}).Run(Services{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, MemoryMarker)); !os.IsNotExist(err) {
		t.Fatalf("a Profile with no memory left a marker: %v", err)
	}
}

func TestPrepareRefusesAMissingDirectoryItDoesNotOwn(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "gone")
	_, err := PrepareRequest{Profile: Profile{MemoryProject: "ops"}, Dir: dir}.Run(Services{})
	if err == nil || !strings.Contains(err.Error(), dir) {
		t.Fatalf("err = %v; want the directory named", err)
	}
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Fatalf("a directory the Profile does not own was created")
	}
}

func TestPrepareResolvesTheSpecializationAfterTheMemory(t *testing.T) {
	root := t.TempDir()
	skill := filepath.Join(root, "ops", SkillFile)
	if err := os.MkdirAll(filepath.Dir(skill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := PrepareRequest{Profile: Profile{Specialization: "ops"}, Dir: t.TempDir(), Specialize: true, Roots: []string{root}}.Run(Services{})
	if err != nil || got.(Prepared).Skill != skill {
		t.Fatalf("prepared = %+v, %v; want %s", got, err, skill)
	}
	if _, err := (PrepareRequest{Profile: Profile{Specialization: "missing"}, Dir: t.TempDir(), Specialize: true, Roots: []string{root}}).Run(Services{}); err == nil {
		t.Fatal("a Specialization with no skill file prepared")
	}
}

func TestBindMemoryKeepsAMarkerThatNamesTheProject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, MemoryMarker)
	own := "# the owner's\nworkspace = \"mine\"\nproject = \"ops\"\n"
	if err := os.WriteFile(path, []byte(own), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := BindMemory(dir, "ops"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != own {
		t.Fatalf("a marker naming the same project was rewritten:\n%s", data)
	}
}

func TestBindMemoryRenamesTheProjectAndKeepsTheWorkspace(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, MemoryMarker), []byte("workspace = \"mine\"\nproject = \"old\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := BindMemory(dir, "ops"); err != nil {
		t.Fatal(err)
	}
	if m := readMarker(t, dir); m != (marker{Workspace: "mine", Project: "ops"}) {
		t.Fatalf("marker = %+v", m)
	}
}

func TestBindMemoryLeavesAMarkerItCannotRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, MemoryMarker)
	if err := os.WriteFile(path, []byte("project = "), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := BindMemory(dir, "ops"); err == nil {
		t.Fatal("an unreadable marker was overwritten")
	}
	if data, _ := os.ReadFile(path); string(data) != "project = " {
		t.Fatalf("marker = %q", data)
	}
}
