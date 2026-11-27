package bots

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SkillFile is the file a skill directory holds its instructions in.
const SkillFile = "SKILL.md"

// SkillRoots are the directories a Specialization is looked up in, first
// match wins: the Fractory repository's own skills, then the owner's global
// ones. A root whose base did not resolve is left out.
func SkillRoots(factory, home string) []string {
	var roots []string
	if factory != "" {
		roots = append(roots, filepath.Join(factory, ".agents", "skills"))
	}
	if home != "" {
		roots = append(roots, filepath.Join(home, ".claude", "skills"))
	}
	return roots
}

// ResolveSkill is the path of the SKILL.md the Specialization name names,
// from the first root that holds one. The file is never read: a Bot is
// handed the path, so the next one gets whatever the skill says by then.
func ResolveSkill(name string, roots []string) (string, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("the Specialization %q is not a skill name", name)
	}
	for _, root := range roots {
		path := filepath.Join(root, name, SkillFile)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path, nil
		}
	}
	searched := make([]string, len(roots))
	for i, root := range roots {
		searched[i] = filepath.Join(root, name, SkillFile)
	}
	return "", fmt.Errorf("the Specialization %q has no %s: looked for %s", name, SkillFile, strings.Join(searched, ", "))
}

// SpecializeRequest finds the SKILL.md a Profile's Specialization names, on
// the effect lane because it reads the disk.
type SpecializeRequest struct {
	Profile Profile
	Roots   []string
}

func (SpecializeRequest) EffectName() string { return "Bot Specialization lookup" }

// Run is the path of the skill file.
func (r SpecializeRequest) Run(Services) (any, error) {
	return ResolveSkill(r.Profile.Specialization, r.Roots)
}

// SpecializedFlags are the Profile's own flags followed by flag and the
// skill's path, already quoted for the pane's shell.
func SpecializedFlags(own, flag, quotedPath string) string {
	return strings.TrimSpace(strings.TrimSpace(own) + " " + flag + " " + quotedPath)
}
