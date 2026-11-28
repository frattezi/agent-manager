package overview

import (
	"cmp"
	"slices"
	"time"

	"github.com/YoanWai/agent-manager/internal/ui/runs"
)

// Latest is how many finished Runs a Project block names.
const Latest = 3

// Bucket is the Runs that finished in one period, by outcome.
type Bucket struct {
	Start      time.Time
	Done       int
	Failed     int
	NeedsHuman int
}

// Total is every Run the bucket counts.
func (b Bucket) Total() int { return b.Done + b.Failed + b.NeedsHuman }

// Funnel counts Tickets by what their latest Run's pull request did. A pull
// request that is merged or closed was opened, so the stages nest.
type Funnel struct {
	Tickets int
	Opened  int
	Merged  int
	Closed  int
}

// Finished is a finished Run as the Project block names it.
type Finished struct {
	Title   string
	Summary string
	Status  string
}

// Project is one Project's block.
type Project struct {
	Name string
	// Active is the newest update of any of its Runs, which orders the blocks.
	Active time.Time
	// Mode, Base, TicketGate and Merge are read from its latest Run; a rule
	// that cannot be read is runs.Unknown.
	Mode       string
	Base       string
	TicketGate string
	Merge      string

	Queued, Running, NeedsHuman int

	// Hourly says Buckets are hours; otherwise days.
	Hourly  bool
	Buckets []Bucket
	Funnel  Funnel
	// Durations are created-to-finished, oldest finish first.
	Durations []time.Duration
	Latest    []Finished
	// Tree is the Blocker Tree's roots: every unfinished Ticket and the Tickets
	// that wait on each other.
	Tree []*Node
	// Usual is the median of Durations, the usual Run time; zero with none.
	Usual time.Duration
}

// Runs is the Runs Overview.
type Runs struct {
	Window   Window
	Projects []Project
}

// NoProject names the Runs whose record carries no checkout path.
const NoProject = "(no project)"

// Compute builds the Runs Overview at now. A Project is listed when it has a
// Run updated inside the window; its in-flight counts ignore the window. A Run
// without finishedAt is in no bucket and has no duration, and a Ticket without
// prState reaches no funnel stage past its pull request URL. A Ticket whose latest Run is done or marked solved is
// finished. A Project with an
// unfinished Blocker Tree is listed whatever the window.
func Compute(list []runs.Record, window Window, now time.Time, held []runs.HeldTicket, solved map[string]bool) Runs {
	byProject := map[string][]runs.Record{}
	var names []string
	for _, run := range list {
		name := cmp.Or(run.Project(), NoProject)
		if _, seen := byProject[name]; !seen {
			names = append(names, name)
		}
		byProject[name] = append(byProject[name], run)
	}
	out := Runs{Window: window}
	for _, name := range names {
		if project, ok := computeProject(name, byProject[name], window, now, held, solved); ok {
			out.Projects = append(out.Projects, project)
		}
	}
	slices.SortStableFunc(out.Projects, func(a, b Project) int {
		if c := b.Active.Compare(a.Active); c != 0 {
			return c
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return out
}

func computeProject(name string, all []runs.Record, window Window, now time.Time, held []runs.HeldTicket, solved map[string]bool) (Project, bool) {
	project := Project{Name: name, Hourly: window == Window24h}
	var inside []runs.Record
	var baseAt time.Time
	newest := all[0]
	for _, run := range all {
		if run.UpdatedAt.After(project.Active) {
			project.Active = run.UpdatedAt
		}
		if run.UpdatedAt.After(newest.UpdatedAt) {
			newest = run
		}
		if window.Contains(run.UpdatedAt, now) {
			inside = append(inside, run)
		}
	}
	for _, run := range latestRuns(all) {
		if solved[run.ID] {
			continue
		}
		switch run.Status {
		case runs.StatusQueued:
			project.Queued++
		case runs.StatusRunning:
			project.Running++
		case runs.StatusNeedsHuman:
			project.NeedsHuman++
		}
	}
	project.Tree = blockerTree(name, all, held, window, now, solved)
	if len(inside) == 0 && len(project.Tree) == 0 {
		return Project{}, false
	}
	// A Run that does not say its Mode or base leaves it to the newest that does.
	ruled := newest
	for _, run := range all {
		if run.EffectiveMode() != "" && (ruled.EffectiveMode() == "" || run.UpdatedAt.After(ruled.UpdatedAt)) {
			ruled = run
		}
		if run.Base != "" && (project.Base == "" || run.UpdatedAt.After(baseAt)) {
			project.Base, baseAt = run.Base, run.UpdatedAt
		}
	}
	project.Mode = cmp.Or(ruled.EffectiveMode(), runs.Unknown)
	rules := ruled.Rules()
	project.TicketGate = rules.TicketGate
	project.Merge = rules.Merge

	var finished []runs.Record
	for _, run := range inside {
		if run.FinishedAt.IsZero() {
			continue
		}
		switch run.Status {
		case runs.StatusDone, runs.StatusFailed, runs.StatusNeedsHuman:
			finished = append(finished, run)
		}
	}
	slices.SortStableFunc(finished, func(a, b runs.Record) int { return a.FinishedAt.Compare(b.FinishedAt) })
	for _, run := range finished {
		if !run.CreatedAt.IsZero() && !run.FinishedAt.Before(run.CreatedAt) {
			project.Durations = append(project.Durations, run.FinishedAt.Sub(run.CreatedAt))
		}
	}
	if n := len(project.Durations); n > 0 {
		sorted := slices.Clone(project.Durations)
		slices.Sort(sorted)
		project.Usual = (sorted[(n-1)/2] + sorted[n/2]) / 2
	}
	project.Buckets = buckets(finished, window, now)
	for i := len(finished) - 1; i >= 0 && len(project.Latest) < Latest; i-- {
		run := finished[i]
		summary := ""
		if run.Result != nil {
			summary = run.Result.Summary
		}
		project.Latest = append(project.Latest, Finished{Title: cmp.Or(run.RunTitle(), run.TicketRef), Summary: summary, Status: run.Status})
	}
	project.Funnel = funnel(inside)
	return project, true
}

// funnel counts each Ticket once, by its latest Run.
func funnel(inside []runs.Record) Funnel {
	latest := map[string]runs.Record{}
	for _, run := range inside {
		if held, ok := latest[run.TicketRef]; !ok || created(run).After(created(held)) {
			latest[run.TicketRef] = run
		}
	}
	out := Funnel{Tickets: len(latest)}
	for _, run := range latest {
		switch run.PRState {
		case "MERGED":
			out.Merged++
		case "CLOSED":
			out.Closed++
		}
		if run.PRURL != "" || run.PRState != "" {
			out.Opened++
		}
	}
	return out
}

// latestRuns is each Ticket's latest Run.
func latestRuns(all []runs.Record) map[string]*runs.Record {
	latest := map[string]*runs.Record{}
	for i := range all {
		run := &all[i]
		if cur, ok := latest[run.TicketRef]; !ok || created(*run).After(created(*cur)) {
			latest[run.TicketRef] = run
		}
	}
	return latest
}

func created(r runs.Record) time.Time {
	if r.CreatedAt.IsZero() {
		return r.UpdatedAt
	}
	return r.CreatedAt
}

// buckets lays finished Runs (oldest finish first) on hours for 24h and days
// otherwise, ending at now's period. All starts at the first finish.
func buckets(finished []runs.Record, window Window, now time.Time) []Bucket {
	if len(finished) == 0 {
		return nil
	}
	step := func(t time.Time, n int) time.Time {
		if window == Window24h {
			return t.Add(time.Duration(n) * time.Hour)
		}
		return t.AddDate(0, 0, n)
	}
	floor := func(t time.Time) time.Time {
		if window == Window24h {
			return t.Truncate(time.Hour)
		}
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}
	end := floor(now)
	var first time.Time
	switch window {
	case Window24h:
		first = step(end, -23)
	case Window7d:
		first = step(end, -6)
	case Window30d:
		first = step(end, -29)
	default:
		first = floor(finished[0].FinishedAt)
	}
	var out []Bucket
	for start := first; !start.After(end); start = step(start, 1) {
		out = append(out, Bucket{Start: start})
	}
	for _, run := range finished {
		i := 0
		for i+1 < len(out) && !run.FinishedAt.Before(out[i+1].Start) {
			i++
		}
		if run.FinishedAt.Before(out[0].Start) {
			continue
		}
		switch run.Status {
		case runs.StatusDone:
			out[i].Done++
		case runs.StatusFailed:
			out[i].Failed++
		default:
			out[i].NeedsHuman++
		}
	}
	return out
}
