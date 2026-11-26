package rebuild

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/YoanWai/agent-manager/internal/atomicfile"
)

// Kept is how many archived builds the versions store holds.
const Kept = 3

const manifestName = "build.json"

// Entry is one archived build: its directory under the versions store, the
// binary inside it and the time it was installed.
type Entry struct {
	Version     string    `json:"version"`
	Binary      string    `json:"binary"`
	InstalledAt time.Time `json:"installed_at"`
	Dir         string    `json:"-"`
}

// VersionsDir is the versions store beside the manager's state.
func VersionsDir(configDir string) string {
	return filepath.Join(configDir, "versions")
}

// Archive copies the binary at target into the versions store under its
// version stamp, with the time it was installed: the file's modification
// time, which the install that put it there set. A build archived again
// replaces its earlier copy.
func Archive(versionsDir, target, version string) (Entry, error) {
	info, err := os.Stat(target)
	if err != nil {
		return Entry{}, err
	}
	entry := Entry{
		Version:     version,
		Binary:      filepath.Base(target),
		InstalledAt: info.ModTime().UTC(),
		Dir:         filepath.Join(versionsDir, entryName(version)),
	}
	if err := os.MkdirAll(entry.Dir, 0o755); err != nil {
		return Entry{}, err
	}
	copied, err := copyFile(target, entry.Dir, "."+entry.Binary+"-*", info.Mode().Perm())
	if err != nil {
		return Entry{}, err
	}
	if err := os.Rename(copied, filepath.Join(entry.Dir, entry.Binary)); err != nil {
		os.Remove(copied)
		return Entry{}, err
	}
	manifest, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return Entry{}, err
	}
	if err := atomicfile.WriteFile(filepath.Join(entry.Dir, manifestName), append(manifest, '\n'), 0o644); err != nil {
		return Entry{}, err
	}
	return entry, nil
}

// Entries lists the archived builds, the most recently installed first.
// A directory without a readable manifest is not one this package wrote,
// so it is left out and left alone.
func Entries(versionsDir string) ([]Entry, error) {
	dirs, err := os.ReadDir(versionsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []Entry
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		path := filepath.Join(versionsDir, dir.Name())
		data, err := os.ReadFile(filepath.Join(path, manifestName))
		if err != nil {
			continue
		}
		var entry Entry
		if json.Unmarshal(data, &entry) != nil {
			continue
		}
		entry.Dir = path
		entries = append(entries, entry)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if !entries[i].InstalledAt.Equal(entries[j].InstalledAt) {
			return entries[i].InstalledAt.After(entries[j].InstalledAt)
		}
		return entries[i].Dir > entries[j].Dir
	})
	return entries, nil
}

// Prune removes every archived build past the keep most recently
// installed.
func Prune(versionsDir string, keep int) error {
	entries, err := Entries(versionsDir)
	if err != nil || len(entries) <= keep {
		return err
	}
	var errs []error
	for _, entry := range entries[keep:] {
		errs = append(errs, os.RemoveAll(entry.Dir))
	}
	return errors.Join(errs...)
}

// entryName turns a version stamp into one path element. A stamp is
// git describe output or "dev", so anything outside it becomes '_'.
func entryName(version string) string {
	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_', r == '+':
			return r
		}
		return '_'
	}, version)
	if name == "" || name == "." || name == ".." {
		return "_"
	}
	return name
}
