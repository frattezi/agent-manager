package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/YoanWai/agent-manager/internal/ui/runs"
)

const usageRunsResolve = "runs resolve <run-id|ticket-ref> [--undo] [--json]"

func runsSection() section {
	return groupSection("Fractory Runs", "runs", "the Tickets the Runs rail lists from Fractory's Run registry", []command{
		{
			name:  "resolve",
			usage: usageRunsResolve,
			about: "mark a Ticket solved once its failure is handled by hand, so the rail lists it with the finished Tickets until a newer Run; --undo lists it again",
			run: func(args []string, _ func() string, _ string) error {
				return runRunsResolve(os.Stdout, runs.Dir(), args, time.Now())
			},
		},
	})
}

type resolveResult struct {
	Ticket  string `json:"ticket"`
	Run     string `json:"run"`
	Solved  bool   `json:"solved"`
	Changed bool   `json:"changed"`
}

func runRunsResolve(out io.Writer, dir string, args []string, now time.Time) error {
	set := newFlagSet(usageRunsResolve)
	undo := set.Bool("undo", false, "unmark the Ticket, so the rail lists it again")
	asJSON := jsonFlag(set)
	operands, err := parseCommand(out, set, args, 1, 1)
	if err != nil {
		return err
	}
	if dir == "" {
		return fmt.Errorf("no Run registry: set FRACTORY_HOME or HOME")
	}
	list, err := runs.Load(dir)
	if err != nil {
		return err
	}
	ticket, ok := runs.FindTicket(runs.Tickets(list, nil, nil), operands[0])
	if !ok {
		return fmt.Errorf("no Run or Ticket %q in %s", operands[0], dir)
	}
	result := resolveResult{Ticket: ticket.Ref, Run: ticket.Latest().ID, Solved: !*undo}
	var human string
	if *undo {
		result.Changed, err = runs.UnmarkSolved(dir, result.Run)
		human = ticket.Ref + " is no longer marked solved"
		if !result.Changed {
			human = ticket.Ref + " was not marked solved"
		}
	} else {
		result.Changed, err = runs.MarkSolved(dir, result.Run, now)
		human = ticket.Ref + " is marked solved"
		if !result.Changed {
			human = ticket.Ref + " was already marked solved"
		}
	}
	if err != nil {
		return err
	}
	return emit(out, *asJSON, result, human)
}
