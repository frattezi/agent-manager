package overview

import (
	"cmp"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/YoanWai/agent-manager/internal/ui/runs"
)

// Node is one Ticket in a Blocker Tree.
type Node struct {
	Ref string
	// Run is the Ticket's latest Run; nil for a Ticket known only as a blocker.
	Run *runs.Record
	// Reason is why the Ticket above holds this one, when it is held.
	Reason string
	// Join marks a Ticket's later appearances under its other blockers; it is
	// drawn without its dependents, which sit under the first appearance.
	Join bool
	// Cycle marks a Ticket whose blockers lead back to it.
	Cycle bool
	// Elapsed is how long a running Ticket's Run has run.
	Elapsed  time.Duration
	Children []*Node
}

// edge is a Ticket's wait on one blocker; reason is Fractory's, when held.
type edge struct{ reason string }

// blockerTree is the Project's Blocker Tree: every unfinished Ticket and every
// Ticket that has a blocker or is one, a lone one a root of its own, each under every Ticket it waits on. Blockers that form a cycle are
// drawn once, as roots. A top-level tree is kept while any Ticket in it is
// unfinished, else while its newest Run is inside the window.
func blockerTree(project string, all []runs.Record, held []runs.HeldTicket, window Window, now time.Time, solved map[string]bool) []*Node {
	latest := latestRuns(all)
	blockers := map[string]map[string]edge{} // ticket -> blocker -> edge
	for ref, run := range latest {
		for _, b := range run.BlockedBy {
			if blockers[ref] == nil {
				blockers[ref] = map[string]edge{}
			}
			blockers[ref][b] = edge{}
		}
	}
	for _, h := range held {
		if len(h.Blockers) == 0 || filepath.Base(h.RepoPath) != project {
			continue
		}
		blockers[h.TicketRef] = map[string]edge{}
		for _, b := range h.Blockers {
			blockers[h.TicketRef][b] = edge{h.Reason(b)}
		}
	}
	dependents := map[string][]string{}
	inTree := map[string]bool{}
	for ref, run := range latest {
		if !finished(run, solved) {
			inTree[ref] = true
		}
	}
	newNode := func(ref, reason string) *Node {
		node := &Node{Ref: ref, Run: latest[ref], Reason: reason}
		if node.Run != nil && node.Run.Status == runs.StatusRunning {
			node.Elapsed = max(now.Sub(created(*node.Run)), 0)
		}
		return node
	}
	for ref, set := range blockers {
		inTree[ref] = true
		for b := range set {
			inTree[b] = true
			dependents[b] = append(dependents[b], ref)
		}
	}
	for _, list := range dependents {
		slices.SortFunc(list, compareRefs)
	}
	var roots []string
	for ref := range inTree {
		if len(blockers[ref]) == 0 {
			roots = append(roots, ref)
		}
	}
	slices.SortFunc(roots, compareRefs)

	drawn := map[string]bool{}
	var grow func(ref, reason string, skip map[string]bool) *Node
	grow = func(ref, reason string, skip map[string]bool) *Node {
		node := newNode(ref, reason)
		if drawn[ref] {
			node.Join = true
			return node
		}
		drawn[ref] = true
		for _, child := range dependents[ref] {
			if !skip[child] {
				node.Children = append(node.Children, grow(child, blockers[child][ref].reason, skip))
			}
		}
		return node
	}
	var trees []*Node
	for _, ref := range roots {
		trees = append(trees, grow(ref, "", nil))
	}
	// What no root reaches waits on a cycle, or on something that does.
	var cyclic []string
	for ref := range inTree {
		if !drawn[ref] && reaches(ref, ref, blockers, map[string]bool{}) {
			cyclic = append(cyclic, ref)
		}
	}
	slices.SortFunc(cyclic, compareRefs)
	members := map[string]bool{}
	for _, ref := range cyclic {
		members[ref] = true
	}
	for _, ref := range cyclic {
		drawn[ref] = true
	}
	for _, ref := range cyclic {
		node := newNode(ref, "")
		node.Cycle = true
		for _, child := range dependents[ref] {
			if !members[child] {
				node.Children = append(node.Children, grow(child, blockers[child][ref].reason, members))
			}
		}
		trees = append(trees, node)
	}

	var kept []*Node
	for _, tree := range trees {
		if keepTree(tree, window, now, solved) {
			kept = append(kept, tree)
		}
	}
	return kept
}

// reaches reports whether to is a blocker of from, directly or through others.
func reaches(from, to string, blockers map[string]map[string]edge, seen map[string]bool) bool {
	for b := range blockers[from] {
		if b == to {
			return true
		}
		if !seen[b] {
			seen[b] = true
			if reaches(b, to, blockers, seen) {
				return true
			}
		}
	}
	return false
}

// finished is a Ticket whose latest Run is done or marked solved.
func finished(run *runs.Record, solved map[string]bool) bool {
	return run.Status == runs.StatusDone || solved[run.ID]
}

func keepTree(root *Node, window Window, now time.Time, solved map[string]bool) bool {
	var newest time.Time
	unfinished := false
	var walk func(n *Node)
	walk = func(n *Node) {
		if n.Run == nil || !finished(n.Run, solved) {
			unfinished = true
		} else if n.Run.UpdatedAt.After(newest) {
			newest = n.Run.UpdatedAt
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	return unfinished || window.Contains(newest, now)
}

// compareRefs orders Tickets by number, then by Ref.
func compareRefs(a, b string) int {
	return cmp.Or(cmp.Compare(refNumber(a), refNumber(b)), cmp.Compare(a, b))
}

func refNumber(ref string) int {
	n, _ := strconv.Atoi(ref[strings.LastIndex(ref, "#")+1:])
	return n
}
