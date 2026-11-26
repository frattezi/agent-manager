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
	Key          string    `json:"key"`
	Trigger      Trigger   `json:"trigger"`
	CreatedAt    time.Time `json:"createdAt"`
	TicketRef    string    `json:"ticketRef"`
	Status       string    `json:"status"`
	RepoPath     string    `json:"repoPath"`
	Branch       string    `json:"branch"`
	WorktreePath string    `json:"worktreePath"`
	PRURL        string    `json:"prUrl"`
	LogPath      string    `json:"logPath"`
	Result       *Result   `json:"result"`
	UpdatedAt    time.Time `json:"updatedAt"`
	// Mode, Flow and Agents are optional: older records do not carry them.
	Mode   string   `json:"mode"`
	Flow   string   `json:"flow"`
	Agents []string `json:"agents"`
	// CleanedAt is when Fractory's cleanup removed the Run's worktree after
	// its PR merged; zero while it has not.
	CleanedAt time.Time `json:"cleanedAt"`
	// Config is the repo's `.fractory.json`; Load reads it, nil when unreadable.
	Config *RepoConfig `json:"-"`
	// Base is the branch the Run is reviewed against; Load reads it from the
	// repo, the record does not carry it.
	Base string `json:"-"`
}

// Trigger is what started the Run: manual, ci-failed, review-comment, ...
type Trigger struct {
	Kind string `json:"kind"`
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
	bases := map[string]string{}
	configs := map[string]*RepoConfig{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") || strings.HasSuffix(name, commentsSuffix) || strings.HasSuffix(name, rerunSuffix) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if record, err := Parse(data); err == nil {
			if _, seen := bases[record.RepoPath]; !seen {
				bases[record.RepoPath] = BaseBranch(record.RepoPath)
				configs[record.RepoPath] = ReadRepoConfig(record.RepoPath)
			}
			record.Config = configs[record.RepoPath]
			record.Base = bases[record.RepoPath]
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

// DefaultBase is the base branch of a repo that names none.
const DefaultBase = "main"

// BaseBranch is the `baseBranch` a repo sets in `.fractory.json`, else
// DefaultBase. An unreadable or malformed file is no setting.
func BaseBranch(repoPath string) string {
	if repoPath == "" {
		return DefaultBase
	}
	data, err := os.ReadFile(filepath.Join(repoPath, ".fractory.json"))
	if err != nil {
		return DefaultBase
	}
	var config struct {
		BaseBranch string `json:"baseBranch"`
	}
	if json.Unmarshal(data, &config) != nil || config.BaseBranch == "" {
		return DefaultBase
	}
	return config.BaseBranch
}
