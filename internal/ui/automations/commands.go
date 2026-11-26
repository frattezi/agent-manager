package automations

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// The commands are the one place an Automation is created, changed, paused,
// removed or run by hand. The TUI's dialog and agent-manager automations
// both call them, so every front stores the same values and refuses the
// same ones.

// ErrTaken reports an id another Automation already holds.
var ErrTaken = errors.New("automation id is taken")

// ErrNotFound reports an id no Automation carries.
var ErrNotFound = errors.New("automation does not exist")

// Book is the store the commands read and write Automations in.
type Book interface {
	Store
	AddAutomation(row Row, now time.Time) error
	UpdateAutomation(id, name string, document []byte, now time.Time) error
	SetAutomationEnabled(id string, enabled bool, now time.Time) error
	DeleteAutomation(id string) error
}

// Find takes an id, or a name exactly one Automation carries.
func Find(book Book, key string) (Row, error) {
	if row, found, err := book.AutomationRow(key); err != nil || found {
		return row, err
	}
	rows, err := book.AutomationRows()
	if err != nil {
		return Row{}, err
	}
	var named []Row
	for _, row := range rows {
		if row.Name == key {
			named = append(named, row)
		}
	}
	switch len(named) {
	case 0:
		return Row{}, fmt.Errorf("no Automation has the id or name %q", key)
	case 1:
		return named[0], nil
	}
	ids := make([]string, len(named))
	for i, row := range named {
		ids[i] = row.ID
	}
	return Row{}, fmt.Errorf("%d Automations are named %q; name one by id: %s", len(named), key, strings.Join(ids, ", "))
}

// Admit is every rule a configuration meets before the store keeps it:
// Validate's, and a directory Fractory can run in.
func (c Config) Admit(tools []string) error {
	if err := c.Validate(tools); err != nil {
		return err
	}
	return checkRepository(c.RepoPath)
}

// checkRepository refuses a directory without a readable .fractory.json,
// which is where Fractory reads the image and the rules a run needs.
func checkRepository(dir string) error {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return refuse("repoPath", "%q is not a directory", dir)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".fractory.json"))
	if err != nil {
		return refuse("repoPath", "%q is not a Fractory repository: it has no .fractory.json", dir)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return refuse("repoPath", "%q is not a Fractory repository: its .fractory.json is not a JSON object", dir)
	}
	return nil
}

// Add stores a new Automation, assigning its id unless the configuration
// names one, and returns the configuration as stored.
func Add(book Book, config Config, enabled bool, tools []string, now time.Time) (Config, error) {
	if config.ID == "" {
		config.ID = uuid.NewString()
	}
	if err := config.Admit(tools); err != nil {
		return Config{}, err
	}
	document, err := config.Encode()
	if err != nil {
		return Config{}, err
	}
	err = book.AddAutomation(Row{ID: config.ID, Name: config.Name, Document: document, Enabled: enabled}, now)
	if errors.Is(err, ErrTaken) {
		return Config{}, refuse("id", "%q is already another Automation's", config.ID)
	}
	return config, err
}

// Change replaces the configuration of the Automation id. Its id, enabled
// flag and last fired instant stay as they are.
func Change(book Book, id string, config Config, tools []string, now time.Time) (Config, error) {
	if config.ID != "" && config.ID != id {
		return Config{}, refuse("id", "is %q, but an Automation's id never changes; leave it out or write %q", config.ID, id)
	}
	config.ID = id
	if err := config.Admit(tools); err != nil {
		return Config{}, err
	}
	document, err := config.Encode()
	if err != nil {
		return Config{}, err
	}
	return config, book.UpdateAutomation(id, config.Name, document, now)
}

func SetEnabled(book Book, id string, enabled bool, now time.Time) error {
	return book.SetAutomationEnabled(id, enabled, now)
}

func Remove(book Book, id string) error { return book.DeleteAutomation(id) }

// RunNowResult is what a run now did: the instant it asked for, and
// whether it wrote a request or skipped the instant because an earlier
// firing is still queued or running.
type RunNowResult struct {
	FiredFor time.Time
	Skipped  bool
}

// RunNow asks Fractory to run the Automation id for the current instant,
// outside its schedule. The watermark stays, so the next scheduled instant
// still fires; like a scheduled one, the instant is skipped while an earlier
// firing is unfinished.
func RunNow(services Services, id string, now time.Time) (RunNowResult, error) {
	row, found, err := services.Store.AutomationRow(id)
	if err != nil {
		return RunNowResult{}, err
	}
	if !found {
		return RunNowResult{}, fmt.Errorf("automation %q: %w", id, ErrNotFound)
	}
	if services.RequestsDir == "" {
		return RunNowResult{}, errors.New("no Fractory home to write the request to")
	}
	result := RunNowResult{FiredFor: now.UTC().Truncate(time.Second)}
	requests, _, err := LoadRequests(services.RequestsDir)
	if err != nil {
		return RunNowResult{}, fmt.Errorf("listing Automation requests: %w", err)
	}
	var records []Record
	if services.RecordsDir != "" {
		if records, _, err = LoadRecords(services.RecordsDir); err != nil {
			return RunNowResult{}, fmt.Errorf("listing Automation Runs: %w", err)
		}
	}
	if Busy(requests, records)[id] {
		result.Skipped = true
		return result, nil
	}
	if _, err := WriteRequest(services.RequestsDir, row.Document, result.FiredFor, now); err != nil {
		return RunNowResult{}, fmt.Errorf("writing the request: %w", err)
	}
	return result, nil
}
