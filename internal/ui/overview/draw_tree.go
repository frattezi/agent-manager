package overview

import (
	"github.com/YoanWai/agent-manager/internal/ui/runs"

	"fmt"
	"strconv"
	"strings"
	"time"
)

// tree draws the Blocker Tree: roots flush left, each Ticket under the Tickets
// it waits on. Lines longer than width are cut with an ellipsis; the Overview
// scrolls vertically only.
func (p painter) tree(project Project, width int) []string {
	roots := project.Tree
	if len(roots) == 0 {
		return nil
	}
	rows := []string{"", p.label("Blocker Tree")}
	for _, root := range roots {
		rows = append(rows, p.node(root, "", "", project.Usual, width)...)
	}
	rows = append(rows, "")
	return rows
}

// edges are the branch, last branch, pass-through and blank pieces.
func (p painter) edges() (branch, last, pass, blank string) {
	if p.basic {
		return "|- ", "`- ", "|  ", "   "
	}
	return "├─ ", "└─ ", "│  ", "   "
}

// node draws n on a line led by lead, its reason and children indented by rest.
func (p painter) node(n *Node, lead, rest string, usual time.Duration, width int) []string {
	branch, last, pass, blank := p.edges()
	dim := n.Run == nil || n.Join
	var color string
	text := n.Ref
	if n.Run == nil {
		color = p.palette.Dim
		if number := refNumber(n.Ref); number > 0 {
			text = "#" + strconv.Itoa(number)
		}
		text += " no Run yet"
	} else {
		color = p.status(n.Run.Status)
		title := n.Run.RunTitle()
		if title == "" {
			title = n.Run.TicketRef
		}
		text = strings.TrimSpace(title + " " + n.Run.Number().String())
	}
	textStyle := p.fg(p.palette.Bright)
	if dim {
		textStyle = p.fg(p.palette.Dim)
	}
	line := lead + p.fg(color).Render(p.cell()) + " " + textStyle.Render(text)
	if n.Join {
		line += p.label(" (join)")
	}
	if n.Cycle {
		line += p.fg(p.palette.Errored).Render(" (cycle)")
	}
	rows := []string{fit(line, width)}
	more := pass
	if len(n.Children) == 0 {
		more = blank
	}
	for _, detail := range p.details(n, usual) {
		rows = append(rows, fit(rest+more+detail, width))
	}
	for i, child := range n.Children {
		piece, next := branch, pass
		if i == len(n.Children)-1 {
			piece, next = last, blank
		}
		rows = append(rows, p.node(child, rest+piece, rest+next, usual, width)...)
	}
	return rows
}

// details are the lines under a node: why it is held, how far a running Run has
// got and what it is doing, or what a needs-human Run asks.
func (p painter) details(n *Node, usual time.Duration) []string {
	var out []string
	if n.Reason != "" {
		out = append(out, p.label(n.Reason))
	}
	if n.Run == nil {
		return out
	}
	switch n.Run.Status {
	case runs.StatusRunning:
		out = append(out, p.progress(n.Elapsed, usual))
		if n.Run.LogTail != "" {
			out = append(out, p.label(n.Run.LogTail))
		}
	case runs.StatusNeedsHuman:
		if question, _, _ := strings.Cut(strings.TrimSpace(n.Run.Question()), "\n"); question != "" {
			out = append(out, p.label(question))
		}
	}
	return out
}

// barCells is the width of a progress bar.
const barCells = 10

// progress is a running Run's time against the Project's usual Run time, as a
// bar and in words; with no usual time the elapsed time stands alone.
func (p painter) progress(elapsed, usual time.Duration) string {
	if usual <= 0 {
		return p.fg(p.palette.Working).Render(span(elapsed))
	}
	filled := min(int(int64(barCells)*int64(elapsed)/int64(usual)), barCells)
	empty := "░"
	if p.basic {
		empty = "-"
	}
	bar := p.fg(p.palette.Working).Render(strings.Repeat(p.cell(), filled)) + p.label(strings.Repeat(empty, barCells-filled))
	return bar + " " + fmt.Sprintf("%s of a usual %s", span(elapsed), span(usual))
}
