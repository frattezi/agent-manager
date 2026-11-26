package automations

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Row is one stored Automation as a check reads it: the configuration
// document as the store keeps it, and the state beside it.
type Row struct {
	ID           string
	Document     []byte
	Enabled      bool
	LastFiredFor time.Time
	CreatedAt    time.Time
}

// Store is the store a check reads Automations from and advances their
// watermarks in.
type Store interface {
	AutomationRows() ([]Row, error)
	AutomationRow(id string) (Row, bool, error)
	SetLastFired(id string, firedFor time.Time) error
}

// Services is what a check runs against: the store, where requests go and
// where Fractory writes its records. An empty directory is a Fractory home
// that does not resolve.
type Services struct {
	Store       Store
	RequestsDir string
	RecordsDir  string
}

// CheckRequest checks, at Now, whether any Automation is due, and fires it.
type CheckRequest struct{ Now time.Time }

func (CheckRequest) EffectName() string { return "Automations check" }

func (r CheckRequest) Run(services Services) (any, error) { return Check(services, r.Now) }

// Failure is a firing that did not go through.
type Failure struct {
	Firing
	Reason error
}

// CheckResult is what one check did. Fired holds the instants it wrote a
// request for.
type CheckResult struct {
	Fired      []Firing
	Skipped    []Firing
	Failed     []Failure
	Unreadable []Unreadable
}

// Check fires every Automation due at now. A firing writes its request and
// then advances the watermark, so a check that dies between the two leaves
// a request the next one finds outstanding, or a record for that instant,
// and the instant is never requested twice. A request that cannot be written
// leaves the watermark where it was, and the next check tries again. A busy
// Automation is skipped and its watermark advanced too.
func Check(services Services, now time.Time) (CheckResult, error) {
	rows, err := services.Store.AutomationRows()
	if err != nil {
		return CheckResult{}, fmt.Errorf("reading Automations: %w", err)
	}
	var requests []Request
	var records []Record
	if services.RequestsDir != "" {
		if requests, _, err = LoadRequests(services.RequestsDir); err != nil {
			return CheckResult{}, fmt.Errorf("listing Automation requests: %w", err)
		}
	}
	if services.RecordsDir != "" {
		if records, _, err = LoadRecords(services.RecordsDir); err != nil {
			return CheckResult{}, fmt.Errorf("listing Automation Runs: %w", err)
		}
	}
	var result CheckResult
	stored := make([]Stored, 0, len(rows))
	for _, row := range rows {
		var config Config
		if err := json.Unmarshal(row.Document, &config); err != nil {
			result.Unreadable = append(result.Unreadable, Unreadable{ID: row.ID, Reason: err})
			continue
		}
		stored = append(stored, Stored{Config: config, Enabled: row.Enabled, LastFiredFor: row.LastFiredFor, CreatedAt: row.CreatedAt})
	}
	decision := Due(stored, Busy(requests, records), now)
	result.Unreadable = append(result.Unreadable, decision.Unreadable...)
	recorded := map[Firing]bool{}
	for _, record := range records {
		recorded[Firing{ID: record.AutomationID, FiredFor: record.FiredFor.UTC()}] = true
	}
	for _, firing := range decision.Fire {
		fired, err := fire(services, firing, recorded, now)
		switch {
		case err != nil:
			result.Failed = append(result.Failed, Failure{Firing: firing, Reason: err})
		case fired:
			result.Fired = append(result.Fired, firing)
		}
	}
	for _, firing := range decision.Skipped {
		if err := services.Store.SetLastFired(firing.ID, firing.FiredFor); err != nil {
			result.Failed = append(result.Failed, Failure{Firing: firing, Reason: err})
			continue
		}
		result.Skipped = append(result.Skipped, firing)
	}
	return result, nil
}

// fire writes one request and advances the watermark, reporting whether a
// request was written. The row is read again first, so an Automation disabled,
// deleted or fired by another manager since the listing is left alone.
func fire(services Services, firing Firing, recorded map[Firing]bool, now time.Time) (bool, error) {
	row, ok, err := services.Store.AutomationRow(firing.ID)
	if err != nil {
		return false, fmt.Errorf("reading Automation: %w", err)
	}
	if !ok || !row.Enabled || !row.LastFiredFor.Before(firing.FiredFor) {
		return false, nil
	}
	// Fractory took this instant's request and recorded a run for it; the
	// watermark only missed its advance.
	requested := recorded[Firing{ID: firing.ID, FiredFor: firing.FiredFor.UTC()}]
	if !requested {
		if services.RequestsDir == "" {
			return false, errors.New("no Fractory home to write the request to")
		}
		if _, err := WriteRequest(services.RequestsDir, row.Document, firing.FiredFor, now); err != nil {
			return false, fmt.Errorf("writing the request: %w", err)
		}
	}
	if err := services.Store.SetLastFired(firing.ID, firing.FiredFor); err != nil {
		return false, fmt.Errorf("the request was written, but advancing the watermark failed: %w", err)
	}
	return !requested, nil
}
