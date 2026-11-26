package runs

import (
	"slices"
	"strings"
)

// Ticket is every Run of one Ticket, newest first. The rail shows one row per
// Ticket: its latest Run.
type Ticket struct {
	Ref string
	// Runs is newest first by creation time.
	Runs []Record
	// Reruns counts the Runs a Re-run started; follow-ups are not Re-runs.
	Reruns int
	// Pending is true while the latest Run has a Re-run request and no Re-run
	// Run exists for it yet.
	Pending bool
	// Solved is true while the owner has marked the latest Run's Ticket
	// handled by hand.
	Solved bool
}

// Latest is the Ticket's newest Run.
func (t Ticket) Latest() Record { return t.Runs[0] }

// Finished reports whether the Ticket needs nothing more: the owner marked it
// solved, or its latest Run is done and Fractory has cleaned it up.
func (t Ticket) Finished() bool {
	if t.Solved {
		return true
	}
	latest := t.Latest()
	return latest.Status == StatusDone && !latest.CleanedAt.IsZero()
}

// Listed is the Tickets the rail lists, in order, and how many are finished.
// Finished Tickets are left out unless showFinished.
func Listed(tickets []Ticket, showFinished bool) (listed []Ticket, finished int) {
	for _, ticket := range tickets {
		if ticket.Finished() {
			finished++
			if !showFinished {
				continue
			}
		}
		listed = append(listed, ticket)
	}
	return listed, finished
}

func (r Record) created() int64 {
	if r.CreatedAt.IsZero() {
		return r.UpdatedAt.UnixNano()
	}
	return r.CreatedAt.UnixNano()
}

// Tickets groups Runs by Ticket. Tickets keep the order their first Run has in
// list, which Load sorts active first and then by recent update. requested is
// the set of Run ids with a Re-run request file, solved those with a solved
// mark.
func Tickets(list []Record, requested, solved map[string]bool) []Ticket {
	var tickets []Ticket
	index := map[string]int{}
	keys := map[string]bool{}
	for _, run := range list {
		keys[run.Key] = true
		i, ok := index[run.TicketRef]
		if !ok {
			i = len(tickets)
			index[run.TicketRef] = i
			tickets = append(tickets, Ticket{Ref: run.TicketRef})
		}
		tickets[i].Runs = append(tickets[i].Runs, run)
		if strings.Contains(run.Key, RerunKeyInfix) {
			tickets[i].Reruns++
		}
	}
	for i := range tickets {
		runs := tickets[i].Runs
		slices.SortStableFunc(runs, func(a, b Record) int {
			switch {
			case a.created() > b.created():
				return -1
			case a.created() < b.created():
				return 1
			}
			return 0
		})
		latest := runs[0]
		tickets[i].Pending = requested[latest.ID] && !keys[latest.TicketRef+RerunKeyInfix+latest.ID]
		tickets[i].Solved = solved[latest.ID]
	}
	return tickets
}

// FindTicket is the Ticket an id names: one of its Runs' ids, or its ref.
func FindTicket(tickets []Ticket, id string) (Ticket, bool) {
	for _, ticket := range tickets {
		if ticket.Ref == id || slices.ContainsFunc(ticket.Runs, func(run Record) bool { return run.ID == id }) {
			return ticket, true
		}
	}
	return Ticket{}, false
}
