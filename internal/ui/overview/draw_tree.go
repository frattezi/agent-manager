package overview

import (
	"strconv"
	"strings"
)

// tree draws the Blocker Tree: roots flush left, each Ticket under the Tickets
// it waits on. Lines longer than width are cut with an ellipsis; the Overview
// scrolls vertically only.
func (p painter) tree(roots []*Node, width int) []string {
	if len(roots) == 0 {
		return nil
	}
	rows := []string{"", p.label("Blocker Tree")}
	for _, root := range roots {
		rows = append(rows, p.node(root, "", "", width)...)
	}
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
func (p painter) node(n *Node, lead, rest string, width int) []string {
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
	if n.Reason != "" {
		more := pass
		if len(n.Children) == 0 {
			more = blank
		}
		rows = append(rows, fit(rest+more+p.label(n.Reason), width))
	}
	for i, child := range n.Children {
		piece, next := branch, pass
		if i == len(n.Children)-1 {
			piece, next = last, blank
		}
		rows = append(rows, p.node(child, rest+piece, rest+next, width)...)
	}
	return rows
}
