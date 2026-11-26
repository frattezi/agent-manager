package automations

import (
	"encoding/json"
	"time"
)

// Record statuses that mean an Automation Run has yet to finish.
const (
	StatusQueued  = "queued"
	StatusRunning = "running"
)

// Stored is what the due computation reads about one stored Automation.
type Stored struct {
	Config  Config
	Enabled bool
	// LastFiredFor is the watermark: the instant it last fired for, zero
	// when it never has.
	LastFiredFor time.Time
	CreatedAt    time.Time
}

// Firing is one Automation and the instant it is due for.
type Firing struct {
	ID       string
	FiredFor time.Time
}

// Unreadable is a stored Automation whose schedule cannot be read.
type Unreadable struct {
	ID     string
	Reason error
}

// Decision is what is due at one moment.
type Decision struct {
	Fire       []Firing
	Skipped    []Firing
	Unreadable []Unreadable
}

// Due works out which Automations are due at now. busy holds the ids of
// Automations with an Automation Run outstanding; those are skipped, never
// queued behind it. A due Automation fires once, for the most recent
// instant it missed. One that never fired counts from when it was created,
// so adding it does not set it off.
func Due(automations []Stored, busy map[string]bool, now time.Time) Decision {
	var decision Decision
	for _, a := range automations {
		if !a.Enabled {
			continue
		}
		schedule, err := ParseSchedule(a.Config.Cron, a.Config.Timezone)
		if err != nil {
			decision.Unreadable = append(decision.Unreadable, Unreadable{ID: a.Config.ID, Reason: err})
			continue
		}
		since := a.LastFiredFor
		if since.IsZero() {
			since = a.CreatedAt
		}
		next, ok := schedule.Next(since)
		if !ok || next.After(now) {
			continue
		}
		latest, _ := schedule.Latest(now)
		firing := Firing{ID: a.Config.ID, FiredFor: latest}
		if busy[a.Config.ID] {
			decision.Skipped = append(decision.Skipped, firing)
		} else {
			decision.Fire = append(decision.Fire, firing)
		}
	}
	return decision
}

// Busy is the set of Automations with an Automation Run outstanding: a
// request Fractory has yet to take, or a record still queued or running.
func Busy(requests []Request, records []Record) map[string]bool {
	busy := map[string]bool{}
	for _, request := range requests {
		var config struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(request.Automation, &config) == nil && config.ID != "" {
			busy[config.ID] = true
		}
	}
	for _, record := range records {
		if record.Status == StatusQueued || record.Status == StatusRunning {
			busy[record.AutomationID] = true
		}
	}
	return busy
}
