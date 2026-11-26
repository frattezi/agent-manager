package automations

import (
	"encoding/json"
	"fmt"
	"time"
)

// Finished reports a record that is neither queued nor running.
func (r Record) Finished() bool { return r.Status != StatusQueued && r.Status != StatusRunning }

// Outcome is how the Automation Run went: the result's outcome once it has
// one, its status until then.
func (r Record) Outcome() string {
	if r.Result != nil && r.Result.Outcome != "" {
		return r.Result.Outcome
	}
	return r.Status
}

// Duration is how long the Automation Run took, or has taken so far while
// it runs. It has none before it starts.
func (r Record) Duration(now time.Time) (time.Duration, bool) {
	if r.StartedAt.IsZero() {
		return 0, false
	}
	end := r.UpdatedAt
	if !r.Finished() {
		end = now
	}
	if end.Before(r.StartedAt) {
		return 0, false
	}
	return end.Sub(r.StartedAt), true
}

// FormatDuration is a duration as the rail shows it: 42s, 3m12s, 1h05m.
func FormatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d/time.Second))
	case d < time.Hour:
		return fmt.Sprintf("%dm%02ds", int(d/time.Minute), int(d%time.Minute/time.Second))
	}
	return fmt.Sprintf("%dh%02dm", int(d/time.Hour), int(d%time.Hour/time.Minute))
}

// ByAutomation groups records under the Automation they belong to, keeping
// the order they came in, so a list LoadRecords returned stays newest first.
func ByAutomation(records []Record) map[string][]Record {
	grouped := map[string][]Record{}
	for _, record := range records {
		grouped[record.AutomationID] = append(grouped[record.AutomationID], record)
	}
	return grouped
}

// Describe reads the parts of a stored configuration a view shows, without
// validating it, so a document a later release refuses still lists.
func Describe(data []byte) (Config, error) {
	var config Config
	err := json.Unmarshal(data, &config)
	return config, err
}

// NextFire is the first instant after now the configuration's schedule
// names.
func NextFire(config Config, now time.Time) (time.Time, error) {
	schedule, err := ParseSchedule(config.Cron, config.Timezone)
	if err != nil {
		return time.Time{}, err
	}
	next, ok := schedule.Next(now)
	if !ok {
		return time.Time{}, fmt.Errorf("%q never fires", config.Cron)
	}
	return next, nil
}
