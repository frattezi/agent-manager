package overview

import (
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/ui/runs"
)

var now = time.Date(2026, 11, 28, 12, 30, 0, 0, time.UTC)

func run(id, ticket, project, status string, updatedAgo time.Duration, edit func(*runs.Record)) runs.Record {
	r := runs.Record{ID: id, TicketRef: ticket, RepoPath: "/src/" + project, Status: status, UpdatedAt: now.Add(-updatedAgo), CreatedAt: now.Add(-updatedAgo - time.Hour)}
	if edit != nil {
		edit(&r)
	}
	return r
}

func finishedAgo(d time.Duration) func(*runs.Record) {
	return func(r *runs.Record) { r.FinishedAt = now.Add(-d) }
}

func names(o Runs) []string {
	var out []string
	for _, p := range o.Projects {
		out = append(out, p.Name)
	}
	return out
}

func TestWindowMembership(t *testing.T) {
	list := []runs.Record{
		run("a", "o/a#1", "a", runs.StatusDone, 2*time.Hour, nil),
		run("b", "o/b#1", "b", runs.StatusDone, 3*24*time.Hour, nil),
		run("c", "o/c#1", "c", runs.StatusDone, 20*24*time.Hour, nil),
		run("d", "o/d#1", "d", runs.StatusDone, 90*24*time.Hour, nil),
	}
	for _, tc := range []struct {
		window Window
		want   int
	}{{Window24h, 1}, {Window7d, 2}, {Window30d, 3}, {WindowAll, 4}} {
		if got := len(Compute(list, tc.window, now, nil).Projects); got != tc.want {
			t.Errorf("window %s lists %d Projects, want %d", tc.window.Label(), got, tc.want)
		}
	}
}

func TestInFlightIgnoresWindow(t *testing.T) {
	list := []runs.Record{
		run("a", "o/a#1", "a", runs.StatusDone, time.Hour, nil),
		run("b", "o/a#2", "a", runs.StatusRunning, 10*24*time.Hour, nil),
		run("c", "o/a#3", "a", runs.StatusQueued, 11*24*time.Hour, nil),
		run("d", "o/a#4", "a", runs.StatusNeedsHuman, 12*24*time.Hour, nil),
	}
	o := Compute(list, Window24h, now, nil)
	if len(o.Projects) != 1 {
		t.Fatalf("Projects = %v", names(o))
	}
	p := o.Projects[0]
	if p.Queued != 1 || p.Running != 1 || p.NeedsHuman != 1 {
		t.Errorf("in flight = %d queued %d running %d needs-human, want 1 each", p.Queued, p.Running, p.NeedsHuman)
	}
}

func TestFunnelCountsATicketOnceByItsLatestRun(t *testing.T) {
	list := []runs.Record{
		run("1", "o/a#1", "a", runs.StatusFailed, 5*time.Hour, func(r *runs.Record) { r.CreatedAt = now.Add(-9 * time.Hour) }),
		run("2", "o/a#1", "a", runs.StatusDone, 4*time.Hour, func(r *runs.Record) {
			r.CreatedAt = now.Add(-6 * time.Hour)
			r.PRURL = "https://github.com/o/a/pull/3"
		}),
		run("3", "o/a#1", "a", runs.StatusDone, 3*time.Hour, func(r *runs.Record) {
			r.CreatedAt = now.Add(-5 * time.Hour)
			r.PRURL = "https://github.com/o/a/pull/3"
			r.PRState = "MERGED"
		}),
		run("4", "o/a#2", "a", runs.StatusDone, time.Hour, func(r *runs.Record) { r.PRURL, r.PRState = "u", "CLOSED" }),
		run("5", "o/a#3", "a", runs.StatusDone, time.Hour, func(r *runs.Record) { r.PRURL = "u" }),
		run("6", "o/a#4", "a", runs.StatusFailed, time.Hour, nil),
	}
	got := Compute(list, WindowAll, now, nil).Projects[0].Funnel
	want := Funnel{Tickets: 4, Opened: 3, Merged: 1, Closed: 1}
	if got != want {
		t.Errorf("funnel = %+v, want %+v", got, want)
	}
}

func TestMissingFinishedAtGivesNothing(t *testing.T) {
	list := []runs.Record{
		run("1", "o/a#1", "a", runs.StatusDone, time.Hour, nil),
		run("2", "o/a#2", "a", runs.StatusDone, 2*time.Hour, finishedAgo(time.Hour)),
	}
	p := Compute(list, WindowAll, now, nil).Projects[0]
	if len(p.Durations) != 1 || p.Durations[0] != 2*time.Hour {
		t.Errorf("durations = %v, want only the Run that has finishedAt", p.Durations)
	}
	if len(p.Latest) != 1 || p.Latest[0].Title == "" {
		t.Errorf("latest = %+v, want one", p.Latest)
	}
	total := 0
	for _, b := range p.Buckets {
		total += b.Total()
	}
	if total != 1 {
		t.Errorf("bucketed %d Runs, want 1", total)
	}
}

func TestMissingPRStateReachesNoFunnelStage(t *testing.T) {
	list := []runs.Record{run("1", "o/a#1", "a", runs.StatusDone, time.Hour, nil)}
	if got := Compute(list, WindowAll, now, nil).Projects[0].Funnel; got != (Funnel{Tickets: 1}) {
		t.Errorf("funnel = %+v, want one Ticket and no stage", got)
	}
}

func TestProjectsMostRecentlyActiveFirst(t *testing.T) {
	list := []runs.Record{
		run("1", "o/a#1", "a", runs.StatusDone, 5*time.Hour, nil),
		run("2", "o/b#1", "b", runs.StatusDone, time.Hour, nil),
		run("3", "o/c#1", "c", runs.StatusDone, 3*time.Hour, nil),
		run("4", "o/a#2", "a", runs.StatusRunning, 2*time.Hour, nil),
	}
	got := names(Compute(list, WindowAll, now, nil))
	want := []string{"b", "a", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestLatestThreeFinishedWithSummaries(t *testing.T) {
	var list []runs.Record
	for i := 1; i <= 4; i++ {
		ago := time.Duration(i) * time.Hour
		list = append(list, run(string(rune('a'+i)), "o/a#"+string(rune('0'+i)), "a", runs.StatusDone, ago, func(r *runs.Record) {
			r.FinishedAt = now.Add(-ago)
			r.Title = "Title " + string(rune('0'+i))
			r.Result = &runs.Result{Summary: "did " + string(rune('0'+i))}
		}))
	}
	latest := Compute(list, WindowAll, now, nil).Projects[0].Latest
	if len(latest) != 3 || latest[0].Title != "Title 1" || latest[0].Summary != "did 1" || latest[2].Title != "Title 3" {
		t.Errorf("latest = %+v", latest)
	}
}

func TestBucketsPerWindow(t *testing.T) {
	list := []runs.Record{run("1", "o/a#1", "a", runs.StatusFailed, time.Hour, finishedAgo(30*time.Minute))}
	for window, want := range map[Window]int{Window24h: 24, Window7d: 7, Window30d: 30, WindowAll: 1} {
		p := Compute(list, window, now, nil).Projects[0]
		if len(p.Buckets) != want {
			t.Errorf("window %s has %d buckets, want %d", window.Label(), len(p.Buckets), want)
		}
		if last := p.Buckets[len(p.Buckets)-1]; last.Failed != 1 {
			t.Errorf("window %s: newest bucket = %+v, want the failure", window.Label(), last)
		}
	}
}

func TestWindowStepsAround(t *testing.T) {
	w, seen := WindowAll, []string{}
	for range 5 {
		w = w.Next()
		seen = append(seen, w.Label())
	}
	want := []string{"24h", "7d", "30d", "all", "24h"}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("steps = %v, want %v", seen, want)
		}
	}
}

func TestProjectColourIsStable(t *testing.T) {
	if ColorIndex("fractory", 7) != ColorIndex("fractory", 7) || ColorIndex("x", 7) >= 7 {
		t.Error("colour index unstable or out of range")
	}
}
