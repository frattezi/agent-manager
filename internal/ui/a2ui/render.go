package a2ui

import (
	"strings"

	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	"github.com/charmbracelet/x/ansi"
)

// minColumn is the narrowest a Row's column gets before the Row stacks its
// children instead, so a narrow surface reads top to bottom.
const minColumn = 20

const columnGap = "  "

// Render draws a document as plain text no wider than width, one string
// with a line per row. It reads nothing but its arguments.
func Render(doc Document, width int) string {
	return strings.Join(draw(doc.Root, max(width, 1)), "\n")
}

func draw(n Node, width int) []string {
	if n.Gap {
		return drawGap(n, width)
	}
	return subset[n.Type].draw(n, width, draw)
}

// wrap folds text to width, breaking a word longer than width. Text an
// agent wrote has its control characters escaped, so it is shown and never
// obeyed by the terminal.
func wrap(text string, width int) []string {
	text = presentation.EscapeControls(text)
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		for _, line := range strings.Split(ansi.Wrap(paragraph, width, ""), "\n") {
			lines = append(lines, strings.TrimRight(line, " "))
		}
	}
	return lines
}

// hang wraps text after a prefix, indenting the lines after the first by
// the prefix's width.
func hang(prefix, text string, width int) []string {
	indent := ansi.StringWidth(prefix)
	lines := wrap(text, max(width-indent, 1))
	for i := range lines {
		if i == 0 {
			lines[i] = prefix + lines[i]
		} else {
			lines[i] = strings.Repeat(" ", indent) + lines[i]
		}
	}
	return lines
}

func stack(n Node, width int, child func(Node, int) []string) []string {
	var lines []string
	for _, c := range n.Children {
		lines = append(lines, child(c, width)...)
	}
	return lines
}

func drawGap(n Node, width int) []string {
	label := "skipped " + n.Type
	if n.ID != "" {
		label += ` "` + n.ID + `"`
	}
	return wrap("["+label+": "+n.Reason+"]", width)
}

func drawText(n Node, width int, _ func(Node, int) []string) []string {
	return wrap(n.Text, width)
}

func drawColumn(n Node, width int, child func(Node, int) []string) []string {
	return stack(n, width, child)
}

func drawList(n Node, width int, child func(Node, int) []string) []string {
	if n.Direction == "horizontal" {
		return drawRow(n, width, child)
	}
	return stack(n, width, child)
}

// drawRow sets children side by side in equal columns, or stacks them when
// a column would be narrower than minColumn.
func drawRow(n Node, width int, child func(Node, int) []string) []string {
	count := len(n.Children)
	if count == 0 {
		return nil
	}
	column := (width - len(columnGap)*(count-1)) / count
	if count == 1 || column < minColumn {
		return stack(n, width, child)
	}
	cells := make([][]string, count)
	height := 0
	for i, c := range n.Children {
		cells[i] = child(c, column)
		height = max(height, len(cells[i]))
	}
	lines := make([]string, height)
	for row := range lines {
		parts := make([]string, count)
		for i, cell := range cells {
			if row < len(cell) {
				parts[i] = cell[row]
			}
			if i < count-1 {
				parts[i] = presentation.PadRight(parts[i], column)
			}
		}
		lines[row] = strings.TrimRight(strings.Join(parts, columnGap), " ")
	}
	return lines
}

func drawCard(n Node, width int, child func(Node, int) []string) []string {
	if width < 5 {
		return stack(n, width, child)
	}
	inner := width - 4
	rule := strings.Repeat("─", width-2)
	lines := []string{"+" + rule + "+"}
	for _, line := range stack(n, inner, child) {
		lines = append(lines, "│ "+presentation.PadRight(line, inner)+" │")
	}
	return append(lines, "+"+rule+"+")
}

func drawDivider(_ Node, width int, _ func(Node, int) []string) []string {
	return []string{strings.Repeat("─", width)}
}

func drawButton(n Node, width int, child func(Node, int) []string) []string {
	label := strings.Join(stack(n, max(width-4, 1), child), " ")
	return []string{ansi.Truncate("[ "+label+" ]", width, "…")}
}

func drawTextField(n Node, width int, _ func(Node, int) []string) []string {
	lines := wrap(n.Label, width)
	if n.Value == "" {
		return append(lines, "> "+strings.Repeat("_", max(min(width-2, 24), 1)))
	}
	return append(lines, hang("> ", n.Value, width)...)
}

func drawCheckBox(n Node, width int, _ func(Node, int) []string) []string {
	return hang(mark(n.Checked, true), n.Label, width)
}

func drawChoicePicker(n Node, width int, _ func(Node, int) []string) []string {
	var lines []string
	if n.Label != "" {
		lines = wrap(n.Label, width)
	}
	for _, option := range n.Options {
		lines = append(lines, hang("  "+mark(option.Selected, n.Multiple), option.Label, width)...)
	}
	return lines
}

// mark is a choice's box: square for one of many that may all be chosen,
// round for one of a set that allows only one.
func mark(selected, square bool) string {
	switch {
	case square && selected:
		return "[x] "
	case square:
		return "[ ] "
	case selected:
		return "(*) "
	default:
		return "( ) "
	}
}
