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

// The gutter is the column a form keeps left of every field it answers, where
// the focused one is marked.
const (
	gutterFocus = "› "
	gutterRest  = "  "
	caret       = "_"
)

// Target is what a drawn line belongs to: Field is a field's number, 0 for
// a line no field drew, and Option the 1-based choice of a ChoicePicker the
// line shows, 0 for none.
type Target struct {
	Field  int
	Option int
}

// Focus is the field a form is on, and for a ChoicePicker the choice under
// its cursor, numbered as Target numbers them.
type Focus struct {
	Field  int
	Option int
}

type line struct {
	text string
	at   Target
}

// pen draws a tree. A form pen keeps a gutter beside each field and marks
// the focused one; the plain pen draws the document as it reads.
type pen struct {
	form  bool
	focus Focus
}

// Render draws a document as plain text no wider than width, one string
// with a line per row. It reads nothing but its arguments.
func Render(doc Document, width int) string {
	return strings.Join(texts(pen{}.draw(doc.Root, max(width, 1))), "\n")
}

// RenderForm draws a document as a form the owner answers: each field
// behind a gutter that marks the one focus is on, a caret in a focused text
// field, and the cursor on a focused multiple choice. Beside each line is
// the field and choice it shows, so a click can be turned into one.
func RenderForm(doc Document, width int, focus Focus) ([]string, []Target) {
	lines := pen{form: true, focus: focus}.draw(doc.Root, max(width, 1))
	targets := make([]Target, len(lines))
	for i, l := range lines {
		targets[i] = l.at
	}
	return texts(lines), targets
}

func texts(lines []line) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.text
	}
	return out
}

func plain(texts []string) []line {
	lines := make([]line, len(texts))
	for i, text := range texts {
		lines[i] = line{text: text}
	}
	return lines
}

// drawers is the subset's draw functions, read through a map built at init
// so a layout that draws its children can live in the subset it reads.
var drawers = map[string]func(pen, Node, int) []line{}

func init() {
	for name, c := range subset {
		drawers[name] = c.draw
	}
}

func (p pen) draw(n Node, width int) []line {
	if n.Gap {
		return plain(drawGap(n, width))
	}
	if !p.form || n.Field == 0 {
		return drawers[n.Type](p, n, width)
	}
	inner := max(width-len(gutterRest), 1)
	lines := drawers[n.Type](p, n, inner)
	for i := range lines {
		gutter := gutterRest
		if i == 0 && p.focused(n) {
			gutter = gutterFocus
		}
		lines[i].text = gutter + lines[i].text
		lines[i].at.Field = n.Field
	}
	return lines
}

func (p pen) focused(n Node) bool { return p.form && n.Field != 0 && p.focus.Field == n.Field }

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

func (p pen) stack(n Node, width int) []line {
	var lines []line
	for _, c := range n.Children {
		lines = append(lines, p.draw(c, width)...)
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

func drawText(_ pen, n Node, width int) []line {
	return plain(wrap(n.Text, width))
}

func drawColumn(p pen, n Node, width int) []line {
	return p.stack(n, width)
}

func drawList(p pen, n Node, width int) []line {
	if n.Direction == "horizontal" {
		return drawRow(p, n, width)
	}
	return p.stack(n, width)
}

// drawRow sets children side by side in equal columns, or stacks them when
// a column would be narrower than minColumn. A line side by side belongs to
// the leftmost field it shows.
func drawRow(p pen, n Node, width int) []line {
	count := len(n.Children)
	if count == 0 {
		return nil
	}
	column := (width - len(columnGap)*(count-1)) / count
	if count == 1 || column < minColumn {
		return p.stack(n, width)
	}
	cells := make([][]line, count)
	height := 0
	for i, c := range n.Children {
		cells[i] = p.draw(c, column)
		height = max(height, len(cells[i]))
	}
	lines := make([]line, height)
	for row := range lines {
		parts := make([]string, count)
		for i, cell := range cells {
			if row < len(cell) {
				parts[i] = cell[row].text
				if lines[row].at.Field == 0 {
					lines[row].at = cell[row].at
				}
			}
			if i < count-1 {
				parts[i] = presentation.PadRight(parts[i], column)
			}
		}
		lines[row].text = strings.TrimRight(strings.Join(parts, columnGap), " ")
	}
	return lines
}

func drawCard(p pen, n Node, width int) []line {
	if width < 5 {
		return p.stack(n, width)
	}
	inner := width - 4
	rule := strings.Repeat("─", width-2)
	lines := []line{{text: "+" + rule + "+"}}
	for _, l := range p.stack(n, inner) {
		lines = append(lines, line{text: "│ " + presentation.PadRight(l.text, inner) + " │", at: l.at})
	}
	return append(lines, line{text: "+" + rule + "+"})
}

func drawDivider(_ pen, _ Node, width int) []line {
	return []line{{text: strings.Repeat("─", width)}}
}

func drawButton(p pen, n Node, width int) []line {
	label := strings.Join(texts(p.stack(n, max(width-4, 1))), " ")
	return []line{{text: ansi.Truncate("[ "+label+" ]", width, "…")}}
}

func drawTextField(p pen, n Node, width int) []line {
	lines := wrap(n.Label, width)
	value := n.Value
	if p.focused(n) {
		value += caret
	}
	if value == "" {
		return plain(append(lines, "> "+strings.Repeat("_", max(min(width-2, 24), 1))))
	}
	return plain(append(lines, hang("> ", value, width)...))
}

func drawCheckBox(_ pen, n Node, width int) []line {
	return plain(hang(mark(n.Checked, true), n.Label, width))
}

func drawChoicePicker(p pen, n Node, width int) []line {
	var lines []line
	if n.Label != "" {
		lines = plain(wrap(n.Label, width))
	}
	for i, option := range n.Options {
		indent := "  "
		if p.focused(n) && n.Multiple && p.focus.Option == i+1 {
			indent = gutterFocus
		}
		for _, text := range hang(indent+mark(option.Selected, n.Multiple), option.Label, width) {
			lines = append(lines, line{text: text, at: Target{Option: i + 1}})
		}
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
