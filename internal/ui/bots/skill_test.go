package bots

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillRootsSearchTheFactoryBeforeTheGlobalSkills(t *testing.T) {
	got := SkillRoots("/f", "/h")
	want := []string{filepath.Join("/f", ".agents", "skills"), filepath.Join("/h", ".claude", "skills")}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("roots = %q; want %q", got, want)
	}
	if only := SkillRoots("", "/h"); len(only) != 1 || only[0] != want[1] {
		t.Fatalf("roots with no factory = %q", only)
	}
}

func TestResolveSkillTakesTheFirstRootThatHoldsIt(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	for _, root := range []string{first, second} {
		if err := os.MkdirAll(filepath.Join(root, "ops"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "ops", SkillFile), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	empty := t.TempDir()
	got, err := ResolveSkill("ops", []string{empty, first, second})
	if err != nil || got != filepath.Join(first, "ops", SkillFile) {
		t.Fatalf("ResolveSkill = %q, %v; want the first root's", got, err)
	}
}

func TestResolveSkillRefusesAMissingSkillOrAPath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "dironly", SkillFile), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"absent", "dironly", "../ops", "a/b", "..", ""} {
		if path, err := ResolveSkill(name, []string{root}); err == nil {
			t.Errorf("ResolveSkill(%q) = %q; want an error", name, path)
		}
	}
	_, err := ResolveSkill("absent", []string{root})
	if err == nil || !strings.Contains(err.Error(), filepath.Join(root, "absent", SkillFile)) {
		t.Fatalf("error = %v; want the path it looked for", err)
	}
}

func TestSpecializedFlagsFollowTheProfilesOwn(t *testing.T) {
	if got := SpecializedFlags(" -u ", "--append-system-prompt-file", "'/s/SKILL.md'"); got != "-u --append-system-prompt-file '/s/SKILL.md'" {
		t.Fatalf("flags = %q", got)
	}
	if got := SpecializedFlags("", "--x", "'p'"); got != "--x 'p'" {
		t.Fatalf("flags with none of its own = %q", got)
	}
}
