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
// match wins: each Fractory-managed repository's committed skills, in the
// order given, then the owner's global ones. A root whose base did not
// resolve is left out.
func SkillRoots(repos []string, home string) []string {
	var roots []string
	for _, repo := range repos {
		if repo != "" {
			roots = append(roots, filepath.Join(repo, ".agents", "skills"))
		}
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

// SpecializedFlags are the Profile's own flags followed by flag and the
// skill's path, already quoted for the pane's shell.
func SpecializedFlags(own, flag, quotedPath string) string {
	return strings.TrimSpace(strings.TrimSpace(own) + " " + flag + " " + quotedPath)
}
