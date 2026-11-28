package overview

import (
	"cmp"
	"path/filepath"
	"slices"
	"time"

	"github.com/YoanWai/agent-manager/internal/ui/automations"
)

// Elsewhere groups the Automations whose configuration names no repository.
const Elsewhere = "Elsewhere"

// Ahead is how far the upcoming-fires timeline looks.
const Ahead = 24 * time.Hour

// maxUpcoming bounds the fires one Automation lists, so a every-minute
// schedule costs a bounded loop.
const maxUpcoming = 24 * 60

// Firing is one firing as a block: its status picks the colour.
type Firing struct {
	Status string
}

// Automation is one Automation's row in a group.
type Automation struct {
	Name    string
	Enabled bool
	// Unreadable is set when the stored document cannot be read.
	Unreadable bool
	// Upcoming are how far from now it fires in the next Ahead; none when disabled.
	Upcoming []time.Duration
	// Firings are the last ten firings inside the window, oldest first.
	Firings []Firing
	// Latest is the newest firing inside the window, false when there is none.
	Latest        Firing
	LatestSummary string
	HasLatest     bool
}

// Group is the Automations of one Project, or of Elsewhere.
type Group struct {
	Name string
	// Active is the newest firing of any member inside the window.
	Active      time.Time
	Automations []Automation
}

// Automations is the Automations Overview.
type Automations struct {
	Window Window
	Groups []Group
}

// ComputeAutomations builds the Automations Overview at now. An Automation
// belongs to the Project its repository path names, the rule a Run follows,
// else to Elsewhere, which lists last. Groups order by latest firing. The
// window keeps the firings whose instant is inside it; upcoming fires ignore it.
func ComputeAutomations(entries []automations.Entry, window Window, now time.Time) Automations {
	byName := map[string]*Group{}
	var order []string
	for _, entry := range entries {
		name := Elsewhere
		if entry.Config.RepoPath != "" {
			name = filepath.Base(entry.Config.RepoPath)
		}
		group, ok := byName[name]
		if !ok {
			group = &Group{Name: name}
			byName[name] = group
			order = append(order, name)
		}
		row := Automation{Name: entry.Name, Enabled: entry.Enabled, Unreadable: entry.Err != nil}
		for _, at := range entry.Upcoming(now, Ahead, maxUpcoming) {
			row.Upcoming = append(row.Upcoming, at.Sub(now))
		}
		var inside []automations.Record
		for _, record := range entry.Firings {
			if window.Contains(record.FiredFor, now) {
				inside = append(inside, record)
			}
		}
		for i := len(inside) - 1; i >= 0; i-- {
			row.Firings = append(row.Firings, Firing{Status: inside[i].Status})
		}
		if len(inside) > 0 {
			row.HasLatest = true
			row.Latest = Firing{Status: inside[0].Status}
			if inside[0].Result != nil {
				row.LatestSummary = inside[0].Result.Summary
			}
			if inside[0].FiredFor.After(group.Active) {
				group.Active = inside[0].FiredFor
			}
		}
		group.Automations = append(group.Automations, row)
	}
	out := Automations{Window: window}
	for _, name := range order {
		out.Groups = append(out.Groups, *byName[name])
	}
	slices.SortStableFunc(out.Groups, func(a, b Group) int {
		if (a.Name == Elsewhere) != (b.Name == Elsewhere) {
			if a.Name == Elsewhere {
				return 1
			}
			return -1
		}
		return cmp.Or(b.Active.Compare(a.Active), cmp.Compare(a.Name, b.Name))
	})
	return out
}
