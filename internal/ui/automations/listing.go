package automations

import (
	"cmp"
	"encoding/json"
	"time"
)

// RecentFirings is how many firings an Automation lists, newest first.
const RecentFirings = 10

// Entry is one stored Automation as the view lists it: its configuration,
// its state and its recent firings, newest first. Err is set when the
// stored document cannot be read, so the row still shows.
type Entry struct {
	ID      string
	Name    string
	Config  Config
	Enabled bool
	Err     error
	Firings []Record
}

// Entries pairs each stored Automation with its records, keeping the
// store's order. Records arrive newest first, as LoadRecords returns them.
func Entries(rows []Row, records []Record) []Entry {
	byAutomation := map[string][]Record{}
	for _, record := range records {
		if len(byAutomation[record.AutomationID]) < RecentFirings {
			byAutomation[record.AutomationID] = append(byAutomation[record.AutomationID], record)
		}
	}
	entries := make([]Entry, len(rows))
	for i, row := range rows {
		entry := Entry{ID: row.ID, Name: row.Name, Enabled: row.Enabled, Firings: byAutomation[row.ID]}
		if err := json.Unmarshal(row.Document, &entry.Config); err != nil {
			entry.Err = err
		}
		entry.Name = cmp.Or(entry.Config.Name, entry.Name, entry.ID)
		entries[i] = entry
	}
	return entries
}

// Last is the newest firing, false when the Automation never fired.
func (e Entry) Last() (Record, bool) {
	if len(e.Firings) == 0 {
		return Record{}, false
	}
	return e.Firings[0], true
}

// Next is the next instant after now the Automation fires for, false when it
// is disabled or its schedule cannot be read.
func (e Entry) Next(now time.Time) (time.Time, bool) {
	if !e.Enabled || e.Err != nil {
		return time.Time{}, false
	}
	schedule, err := ParseSchedule(e.Config.Cron, e.Config.Timezone)
	if err != nil {
		return time.Time{}, false
	}
	return schedule.Next(now)
}

// Outcome is the firing's status, or the outcome its result names when that
// says more.
func (r Record) Outcome() string {
	if r.Result != nil && r.Result.Outcome != "" {
		return r.Result.Outcome
	}
	return r.Status
}

// Finished reports whether the firing has stopped running.
func (r Record) Finished() bool {
	return r.Status != StatusQueued && r.Status != StatusRunning
}

// Duration is how long the firing ran: to its last update once finished, to
// now while it runs. False when it has not started.
func (r Record) Duration(now time.Time) (time.Duration, bool) {
	if r.StartedAt.IsZero() {
		return 0, false
	}
	end := now
	if r.Finished() && !r.UpdatedAt.IsZero() {
		end = r.UpdatedAt
	}
	return max(end.Sub(r.StartedAt), 0), true
}
