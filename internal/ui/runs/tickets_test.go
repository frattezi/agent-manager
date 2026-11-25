package runs

import (
	"slices"
	"testing"
	"time"
)

func run(id, ticket, key, status string, created time.Time) Record {
	return Record{ID: id, TicketRef: ticket, Key: key, Status: status, CreatedAt: created, UpdatedAt: created}
}

func TestTicketsGroupByTicketAndCountOnlyReruns(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	list := []Record{
		run("r3", "jira:A-1", "jira:A-1:rerun:r2", "failed", t0.Add(3*time.Hour)),
		run("r1", "jira:A-1", "jira:A-1", "failed", t0.Add(1*time.Hour)),
		run("r2", "jira:A-1", "jira:A-1:rerun:r1", "failed", t0.Add(2*time.Hour)),
		run("f1", "jira:A-1", "jira:A-1:ci:f", "done", t0.Add(4*time.Hour)),
		run("b1", "jira:B-2", "jira:B-2", "done", t0),
	}
	tickets := Tickets(list, nil)
	if len(tickets) != 2 || tickets[0].Ref != "jira:A-1" || tickets[1].Ref != "jira:B-2" {
		t.Fatalf("tickets = %+v", tickets)
	}
	a := tickets[0]
	if a.Latest().ID != "f1" || a.Reruns != 2 {
		t.Fatalf("A latest=%s reruns=%d; want f1 and 2 (follow-up not counted)", a.Latest().ID, a.Reruns)
	}
	var ids []string
	for _, r := range a.Runs {
		ids = append(ids, r.ID)
	}
	if got := ids; got[0] != "f1" || got[1] != "r3" || got[2] != "r2" || got[3] != "r1" {
		t.Fatalf("history = %v, want newest first", got)
	}
	if tickets[1].Reruns != 0 {
		t.Fatalf("B reruns = %d", tickets[1].Reruns)
	}
}

func TestTicketsPendingUntilTheRerunRunExists(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	failed := run("r1", "jira:A-1", "jira:A-1", "failed", t0)
	requested := map[string]bool{"r1": true}
	if got := Tickets([]Record{failed}, requested); !got[0].Pending {
		t.Fatal("request without its Run must be pending")
	}
	rerun := run("r2", "jira:A-1", "jira:A-1:rerun:r1", "queued", t0.Add(time.Minute))
	got := Tickets([]Record{failed, rerun}, requested)
	if got[0].Pending || got[0].Reruns != 1 {
		t.Fatalf("pending=%v reruns=%d; want the Run to replace the pending marker", got[0].Pending, got[0].Reruns)
	}
	if got := Tickets([]Record{failed}, nil); got[0].Pending {
		t.Fatal("no request, no pending")
	}
}

func TestListedHidesOnlyDoneTicketsThatWereCleanedUp(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cleaned := func(r Record) Record { r.CleanedAt = t0.Add(time.Hour); return r }
	list := []Record{
		run("q", "jira:Q", "jira:Q", StatusQueued, t0),
		run("r", "jira:R", "jira:R", StatusRunning, t0),
		cleaned(run("f", "jira:F", "jira:F", StatusFailed, t0)),
		cleaned(run("n", "jira:N", "jira:N", StatusNeedsHuman, t0)),
		run("o", "jira:O", "jira:O", StatusDone, t0),
		cleaned(run("d", "jira:D", "jira:D", StatusDone, t0)),
		// An older cleaned Run does not hide a Ticket whose latest Run is open.
		cleaned(run("x1", "jira:X", "jira:X", StatusDone, t0)),
		run("x2", "jira:X", "jira:X:ci:x", StatusRunning, t0.Add(time.Minute)),
	}
	tickets := Tickets(list, nil)
	refs := func(tickets []Ticket) []string {
		var out []string
		for _, ticket := range tickets {
			out = append(out, ticket.Ref)
		}
		return out
	}
	listed, finished := Listed(tickets, false)
	if got, want := refs(listed), []string{"jira:Q", "jira:R", "jira:F", "jira:N", "jira:O", "jira:X"}; !slices.Equal(got, want) || finished != 1 {
		t.Fatalf("hidden: listed %v finished %d; want %v and 1", got, finished, want)
	}
	listed, finished = Listed(tickets, true)
	if got, want := refs(listed), refs(tickets); !slices.Equal(got, want) || finished != 1 {
		t.Fatalf("shown: listed %v finished %d; want %v and 1", got, finished, want)
	}
	if len(tickets[6].Runs) != 2 {
		t.Fatalf("history of jira:X = %d Runs, want both", len(tickets[6].Runs))
	}
}
