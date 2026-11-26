package rebuild

import (
	"errors"
	"fmt"
	"path/filepath"
)

var (
	// ErrEmptyArchive is a rollback with nothing in the versions store.
	ErrEmptyArchive = errors.New("no archived builds")
	// ErrNoOtherBuild is a rollback whose only archived build is the one
	// installed now.
	ErrNoOtherBuild = errors.New("no archived build other than the one installed")
	// ErrUnknownBuild is a rollback to a version the store does not hold.
	ErrUnknownBuild = errors.New("no archived build has that version")
)

// Pick chooses the archived build a rollback restores from entries, the
// most recently installed first. An empty name picks the newest one whose
// version is not current, the build that ran before the last rebuild.
func Pick(entries []Entry, current, name string) (Entry, error) {
	if len(entries) == 0 {
		return Entry{}, ErrEmptyArchive
	}
	for _, entry := range entries {
		if name == "" && entry.Version != current || name != "" && entry.Version == name {
			return entry, nil
		}
	}
	if name == "" {
		return Entry{}, ErrNoOtherBuild
	}
	return Entry{}, fmt.Errorf("%w: %s", ErrUnknownBuild, name)
}

// Restore copies the archived build over target the way a rebuild
// installs one, so a manager already running keeps the image it started
// from. It leaves the versions store as it is.
func Restore(entry Entry, target string) (string, error) {
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", fmt.Errorf("finding the installed binary: %w", err)
	}
	if err := install(filepath.Join(entry.Dir, entry.Binary), resolved); err != nil {
		return "", fmt.Errorf("installing over %s: %w", resolved, err)
	}
	return resolved, nil
}
