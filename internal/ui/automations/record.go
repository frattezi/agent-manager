package automations

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/YoanWai/agent-manager/internal/fractoryhome"
)

// RecordVersion is the only Automation Run record version this build reads.
const RecordVersion = 1

// Record is version 1 of the Automation Run record Fractory writes. Fields
// it does not name are ignored, so Fractory can add to the contract.
type Record struct {
	Version      int       `json:"version"`
	ID           string    `json:"id"`
	AutomationID string    `json:"automationId"`
	FiredFor     time.Time `json:"firedFor"`
	Status       string    `json:"status"`
	StartedAt    time.Time `json:"startedAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	LogPath      string    `json:"logPath"`
	OutputPath   string    `json:"outputPath"`
	Result       *Result   `json:"result"`
}

// Result is how the Automation Run ended.
type Result struct {
	Outcome string `json:"outcome"`
	Summary string `json:"summary"`
}

// Skip is a file a listing left out, and why.
type Skip struct {
	Path   string
	Reason error
}

// RecordsDir is where Fractory writes records: $FRACTORY_HOME/automation-runs.
func RecordsDir() string { return fractoryhome.Sub("automation-runs") }

// ParseRecord decodes one record, refusing another version and a record
// missing what a row needs.
func ParseRecord(data []byte) (Record, error) {
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, err
	}
	switch {
	case record.Version != RecordVersion:
		return Record{}, fmt.Errorf("record version %d, this manager reads version %d", record.Version, RecordVersion)
	case record.ID == "" || record.AutomationID == "" || record.FiredFor.IsZero() || record.Status == "":
		return Record{}, errors.New("record is missing a required field")
	}
	return record, nil
}

// LoadRecords reads every valid record in dir, newest instant first. A
// missing dir is no records; a file it cannot use is skipped with a reason.
func LoadRecords(dir string) ([]Record, []Skip, error) {
	return loadDir(dir, ParseRecord, func(a, b Record) int {
		return cmp.Or(b.FiredFor.Compare(a.FiredFor), b.UpdatedAt.Compare(a.UpdatedAt), cmp.Compare(a.ID, b.ID))
	})
}

func loadDir[T any](dir string, parse func([]byte) (T, error), order func(a, b T) int) ([]T, []Skip, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var items []T
	var skips []Skip
	for _, entry := range entries {
		name := entry.Name()
		// A leading dot is atomicfile's temporary sibling, not yet a file.
		if entry.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".json") {
			continue
		}
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			skips = append(skips, Skip{Path: path, Reason: err})
			continue
		}
		item, err := parse(data)
		if err != nil {
			skips = append(skips, Skip{Path: path, Reason: err})
			continue
		}
		items = append(items, item)
	}
	slices.SortStableFunc(items, order)
	return items, skips, nil
}
