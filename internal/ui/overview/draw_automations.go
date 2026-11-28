package overview

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/YoanWai/agent-manager/internal/ui/presentation"
)

// DrawAutomations renders the Automations Overview at width, as Draw does the
// Runs Overview: windowKey names the key that steps the window and every line
// is at most width cells wide.
func DrawAutomations(o Automations, width int, windowKey string, mode presentation.GlyphMode, palette Palette) Drawing {
	p := painter{palette: palette, basic: mode == presentation.GlyphModeBasic}
	inner := max(width-2, 10)
	d := Drawing{Lines: []string{p.windowLabel(o.Window, windowKey, inner), ""}}
	if len(o.Groups) == 0 {
		d.Lines = append(d.Lines, p.fg(palette.Dim).Render("No Automations yet"))
		return d
	}
	for _, group := range o.Groups {
		d.Lines = append(d.Lines, p.group(group, o.Window, inner)...)
		d.Lines = append(d.Lines, "")
	}
	return d
}

func (p painter) group(group Group, window Window, width int) []string {
	color := p.palette.Dim
	if swatches := p.palette.Swatches; group.Name != Elsewhere && len(swatches) > 0 {
		color = swatches[ColorIndex(group.Name, len(swatches))]
	}
	body := width - 4
	rows := wrap(p.fg(color).Bold(true).Render(group.Name), body)
	for _, automation := range group.Automations {
		rows = append(rows, "")
		rows = append(rows, p.automation(automation, window, body)...)
	}
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(color)).Padding(0, 1).Width(width - 2)
	return strings.Split(box.Render(strings.Join(rows, "\n")), "\n")
}

func (p painter) automation(a Automation, window Window, width int) []string {
	name, dim := p.fg(p.palette.Bright).Bold(true), p.fg(p.palette.Dim)
	if !a.Enabled {
		name = dim
	}
	title := name.Render(a.Name)
	switch {
	case a.Unreadable:
		title += "  " + dim.Render("unreadable")
	case !a.Enabled:
		title += "  " + dim.Render("disabled")
	}
	rows := wrap(title, width)
	rows = append(rows, p.timeline(a, width))
	rows = append(rows, p.firings(a, window, width))
	if a.HasLatest {
		rows = append(rows, p.excerpt(a.LatestSummary, width)...)
	}
	return rows
}

// timeline lays the next Ahead on one cell per slot, a fire's slot filled.
func (p painter) timeline(a Automation, width int) string {
	const prefix = "Next 24h  "
	if !a.Enabled || a.Unreadable {
		return p.label(prefix) + "no upcoming fires"
	}
	if len(a.Upcoming) == 0 {
		return p.label(prefix) + "no fire in the next 24h"
	}
	slots := min(max(width-len(prefix)-ansi.StringWidth(" 99 fires"), 8), 48)
	hits := make([]bool, slots)
	step := Ahead / time.Duration(slots)
	for _, offset := range a.Upcoming {
		hits[min(int(offset/step), slots-1)] = true
	}
	var strip strings.Builder
	for _, hit := range hits {
		if hit {
			strip.WriteString(p.fg(p.palette.Working).Render(p.cell()))
		} else {
			strip.WriteString(p.label(p.rest()))
		}
	}
	noun := "fires"
	if len(a.Upcoming) == 1 {
		noun = "fire"
	}
	return p.label(prefix) + strip.String() + p.label(fmt.Sprintf(" %d %s", len(a.Upcoming), noun))
}

func (p painter) rest() string {
	if p.basic {
		return "."
	}
	return "·"
}

func (p painter) firings(a Automation, window Window, width int) string {
	const prefix = "Firings   "
	if len(a.Firings) == 0 {
		if window == WindowAll {
			return p.label(prefix) + "never fired"
		}
		return p.label(prefix) + "no firing in the last " + window.Label()
	}
	var blocks strings.Builder
	for _, f := range a.Firings {
		blocks.WriteString(p.fg(p.status(f.Status)).Render(p.cell()))
	}
	return p.label(prefix) + blocks.String() + p.label(" last "+fmt.Sprint(len(a.Firings)))
}
