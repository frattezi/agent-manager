package overview

import (
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/ui/runs"
)

func TestInFlightCountsEachTicketByItsLatestRun(t *testing.T) {
	list := []runs.Record{
		run("a1", "o/api#43", "api", runs.StatusNeedsHuman, 5*time.Hour, nil),
		run("a2", "o/api#43", "api", runs.StatusDone, time.Hour, nil),
		run("b1", "o/api#8", "api", runs.StatusNeedsHuman, time.Hour, nil),
	}
	if got := Compute(list, WindowAll, now, nil, nil).Projects[0].NeedsHuman; got != 1 {
		t.Errorf("NeedsHuman = %d, want 1", got)
	}
}

func TestSolvedLatestRunIsFinished(t *testing.T) {
	list := []runs.Record{
		run("f1", "o/api#1", "api", runs.StatusFailed, 400*time.Hour, nil),
		run("f2", "o/api#2", "api", runs.StatusFailed, 400*time.Hour, blocked("o/api#1")),
	}
	solved := map[string]bool{"f1": true, "f2": true}

	open := Compute(list, Window24h, now, nil, nil).Projects
	if len(open) != 1 || len(open[0].Tree) != 1 {
		t.Fatalf("unsolved failed Tickets should stay in the tree: %+v", open)
	}
	if got := Compute(list, Window24h, now, nil, solved).Projects; len(got) != 0 {
		t.Errorf("solved Tickets outside the window are still listed: %+v", got)
	}
	inside := Compute(list, WindowAll, now, nil, solved).Projects
	if len(inside) != 1 || len(inside[0].Tree) != 1 {
		t.Errorf("solved Tickets inside the window should show like done ones: %+v", inside)
	}

	nh := []runs.Record{run("n1", "o/api#2", "api", runs.StatusNeedsHuman, time.Hour, nil)}
	if got := Compute(nh, WindowAll, now, nil, map[string]bool{"n1": true}).Projects[0].NeedsHuman; got != 0 {
		t.Errorf("solved needs-human still counted: %d", got)
	}
}

func TestSolvedMarkOnAnOlderRunDoesNotCount(t *testing.T) {
	list := []runs.Record{
		run("o1", "o/api#3", "api", runs.StatusFailed, 400*time.Hour, nil),
		run("o2", "o/api#3", "api", runs.StatusFailed, 300*time.Hour, nil),
	}
	if got := Compute(list, Window24h, now, nil, map[string]bool{"o1": true}).Projects; len(got) != 1 {
		t.Errorf("a mark on a superseded Run finished the Ticket: %+v", got)
	}
}
