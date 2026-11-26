package runs

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YoanWai/agent-manager/internal/atomicfile"
)

const (
	solvedSuffix  = ".solved.json"
	solvedVersion = 1
)

type solvedMark struct {
	Version  int    `json:"version"`
	SolvedAt string `json:"solvedAt"`
}

// SolvedPath is where the manager records that the owner handled a Ticket by
// hand. It is named for the Ticket's latest Run, so a newer Run lists the
// Ticket again.
func SolvedPath(dir, runID string) string {
	return filepath.Join(dir, runID+solvedSuffix)
}

func validRunID(runID string) error {
	if runID == "" || strings.ContainsRune(runID, filepath.Separator) || strings.ContainsRune(runID, '/') {
		return errors.New("invalid run id")
	}
	return nil
}

// MarkSolved records the Run's Ticket as solved. The write is atomic, and a
// mark already on disk is left as it is. It reports whether it wrote one.
func MarkSolved(dir, runID string, now time.Time) (bool, error) {
	if err := validRunID(runID); err != nil {
		return false, err
	}
	path := SolvedPath(dir, runID)
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	out, err := json.Marshal(solvedMark{Version: solvedVersion, SolvedAt: now.UTC().Format(time.RFC3339)})
	if err != nil {
		return false, err
	}
	return true, atomicfile.WriteFile(path, append(out, '\n'), 0o644)
}

// UnmarkSolved removes the Run's solved mark. It reports whether there was one.
func UnmarkSolved(dir, runID string) (bool, error) {
	if err := validRunID(runID); err != nil {
		return false, err
	}
	err := os.Remove(SolvedPath(dir, runID))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// Solved lists the ids of the Runs that carry a solved mark. An unreadable
// dir lists none; the next poll tries again.
func Solved(dir string) map[string]bool {
	return sidecarIDs(dir, solvedSuffix)
}

func sidecarIDs(dir, suffix string) map[string]bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	ids := map[string]bool{}
	for _, entry := range entries {
		if id, ok := strings.CutSuffix(entry.Name(), suffix); ok && !entry.IsDir() {
			ids[id] = true
		}
	}
	return ids
}
