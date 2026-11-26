package runs

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/YoanWai/agent-manager/internal/atomicfile"
)

const (
	rerunSuffix  = ".rerun.json"
	rerunVersion = 1
	// RerunKeyInfix marks a Re-run's key: `<ticketRef>:rerun:<runId>`.
	RerunKeyInfix = ":rerun:"
)

type rerunRequest struct {
	Version     int    `json:"version"`
	RequestedAt string `json:"requestedAt"`
}

// RerunPath is where a Run's Re-run request lives; Fractory's next tick turns
// it into a manual Run on the Ticket's branch.
func RerunPath(dir, runID string) string {
	return filepath.Join(dir, runID+rerunSuffix)
}

// CanRerun reports whether the Run ended in a state a Re-run can retry.
func (r Record) CanRerun() bool {
	return r.Status == StatusFailed || r.Status == StatusNeedsHuman
}

// RequestRerun asks Fractory for a Re-run of the Run. The write is atomic, and
// a request already on disk is left as it is, so a second press changes
// nothing. It reports whether a new request was written.
func RequestRerun(dir, runID string, now time.Time) (bool, error) {
	if err := validRunID(runID); err != nil {
		return false, err
	}
	path := RerunPath(dir, runID)
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	out, err := json.Marshal(rerunRequest{Version: rerunVersion, RequestedAt: now.UTC().Format(time.RFC3339)})
	if err != nil {
		return false, err
	}
	return true, atomicfile.WriteFile(path, append(out, '\n'), 0o644)
}

// Requested lists the ids of the Runs that have a Re-run request file. An
// unreadable dir lists none; the next poll tries again.
func Requested(dir string) map[string]bool {
	return sidecarIDs(dir, rerunSuffix)
}
