package bots

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// MemoryMarker is the file ai-memory resolves a directory's workspace and
// project from, read from the nearest one above the working directory.
const MemoryMarker = ".ai-memory.toml"

// MemoryWorkspace is the workspace a marker the manager writes declares.
// ai-memory ignores a marker that lacks either key, and a Profile names only
// the project, so a Bot's memory lives under this workspace unless the owner
// has already put another in the marker.
const MemoryWorkspace = "bots"

// OwnDir is the directory a Profile with no dir of its own starts its Bots
// in: one per Profile under root, named the way its Bots are.
func OwnDir(root string, p Profile) string {
	return filepath.Join(root, slug(p))
}

// marker is the part of an .ai-memory.toml the binding reads and writes.
type marker struct {
	Workspace string `toml:"workspace"`
	Project   string `toml:"project"`
}

// InRepository reports whether dir is a git repository or lies inside one,
// whose own memory scope a Bot working there inherits.
func InRepository(dir string) (bool, error) {
	for at := filepath.Clean(dir); ; {
		_, err := os.Lstat(filepath.Join(at, ".git"))
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return false, err
		}
		parent := filepath.Dir(at)
		if parent == at {
			return false, nil
		}
		at = parent
	}
}

// BindMemory makes the marker in dir name project, which dir must not be a
// repository's. A marker that already names project is left as it is, and a
// marker naming another keeps its workspace and takes the new project.
func BindMemory(dir, project string) error {
	path := filepath.Join(dir, MemoryMarker)
	current := marker{}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if _, err := toml.Decode(string(data), &current); err != nil {
			return fmt.Errorf("%s is not a memory marker the manager can read: %w", path, err)
		}
		if current.Project == project {
			return nil
		}
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	next := marker{Workspace: current.Workspace, Project: project}
	if next.Workspace == "" {
		next.Workspace = MemoryWorkspace
	}
	var out bytes.Buffer
	if err := toml.NewEncoder(&out).Encode(next); err != nil {
		return err
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

// PrepareRequest readies what a Bot starts on, on the effect lane because it
// touches the disk: Dir, which is the Profile's own directory under the Bots
// root when Owned, created if absent; the memory marker there unless Dir is a
// repository; and, when Specialize, the skill file Roots resolve.
type PrepareRequest struct {
	Profile    Profile
	Dir        string
	Owned      bool
	Specialize bool
	Roots      []string
}

func (PrepareRequest) EffectName() string { return "Bot preparation" }

// Prepared is the directory a Bot starts in and the skill file it loads,
// empty when it loads none.
type Prepared struct {
	Dir   string
	Skill string
}

// Run creates the Bot's own directory, binds its memory and finds its
// Specialization, in that order, and stops at the first that fails.
func (r PrepareRequest) Run(Services) (any, error) {
	if r.Owned {
		if err := os.MkdirAll(r.Dir, 0o755); err != nil {
			return Prepared{}, err
		}
	}
	if info, err := os.Stat(r.Dir); err != nil || !info.IsDir() {
		return Prepared{}, fmt.Errorf("its directory %s does not exist", r.Dir)
	}
	if r.Profile.MemoryProject != "" {
		repo, err := InRepository(r.Dir)
		if err != nil {
			return Prepared{}, err
		}
		if !repo {
			if err := BindMemory(r.Dir, r.Profile.MemoryProject); err != nil {
				return Prepared{}, err
			}
		}
	}
	prepared := Prepared{Dir: r.Dir}
	if r.Specialize {
		skill, err := ResolveSkill(r.Profile.Specialization, r.Roots)
		if err != nil {
			return Prepared{}, err
		}
		prepared.Skill = skill
	}
	return prepared, nil
}
