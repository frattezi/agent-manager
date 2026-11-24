// Package runs reads Fractory's Run registry: a directory of `<id>.json`
// files, one per Run, written by Fractory and only read here. The format is a
// public contract shared with Fractory; see RecordVersion.
package runs

import (
	"cmp"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// RecordVersion is the only RunRecord version this reader understands.
const RecordVersion = 1

// Statuses a Run can be in.
const (
	StatusQueued     = "queued"
	StatusRunning    = "running"
	StatusDone       = "done"
	StatusFailed     = "failed"
	StatusNeedsHuman = "needs-human"
)

// Record is the subset of a RunRecord the manager shows. Fields it does not
// name are ignored, so Fractory can add to the contract without breaking it.
type Record struct {
	Version      int       `json:"version"`
	ID           string    `json:"id"`
	TicketRef    string    `json:"ticketRef"`
	Status       string    `json:"status"`
	Branch       string    `json:"branch"`
	WorktreePath string    `json:"worktreePath"`
	PRURL        string    `json:"prUrl"`
	LogPath      string    `json:"logPath"`
	Result       *Result   `json:"result"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Result is the agent's final answer; the manager reads only a question.
type Result struct {
	Outcome  string `json:"outcome"`
	Question string `json:"question"`
}

// Active reports whether the Run is still going.
func (r Record) Active() bool { return r.Status == StatusQueued || r.Status == StatusRunning }

// Question is what a needs-human Run asks, empty for any other status.
func (r Record) Question() string {
	if r.Status != StatusNeedsHuman || r.Result == nil {
		return ""
	}
	return r.Result.Question
}

// Dir is the registry directory: $FRACTORY_HOME/runs, else ~/.fractory/runs.
func Dir() string {
	home := os.Getenv("FRACTORY_HOME")
	if home == "" {
		user, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		home = filepath.Join(user, ".fractory")
	}
	return filepath.Join(home, "runs")
}

// Parse decodes one record. It refuses another version, and a record missing
// what a row needs, so a half-understood file never reaches the list.
func Parse(data []byte) (Record, error) {
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, err
	}
	switch {
	case record.Version != RecordVersion:
		return Record{}, errors.New("unsupported run record version")
	case record.ID == "" || record.TicketRef == "" || record.LogPath == "" || record.UpdatedAt.IsZero():
		return Record{}, errors.New("run record is missing a required field")
	case !slices.Contains([]string{StatusQueued, StatusRunning, StatusDone, StatusFailed, StatusNeedsHuman}, record.Status):
		return Record{}, errors.New("unknown run status")
	}
	return record, nil
}

// Load reads every valid record in dir, active Runs first and then the most
// recently updated. A missing dir is no Runs; an invalid file is skipped.
func Load(dir string) ([]Record, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []Record
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") || strings.HasSuffix(name, commentsSuffix) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if record, err := Parse(data); err == nil {
			records = append(records, record)
		}
	}
	slices.SortStableFunc(records, func(a, b Record) int {
		if a.Active() != b.Active() {
			if a.Active() {
				return -1
			}
			return 1
		}
		return cmp.Compare(b.UpdatedAt.UnixNano(), a.UpdatedAt.UnixNano())
	})
	return records, nil
}
