package overview

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/NimbleMarkets/ntcharts/barchart"
	"github.com/NimbleMarkets/ntcharts/sparkline"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	"github.com/YoanWai/agent-manager/internal/ui/runs"
)

// Palette is the theme's colours the Overview draws in, as hex.
type Palette struct {
	Swatches                                  []string
	Working, Waiting, Finished, Errored, Idle string
	Bright, Text, Dim                         string
}

// Drawing is what an Overview draws to: its lines, and which one is the window
// label, so a click on it can step the window.
type Drawing struct {
	Lines     []string
	WindowRow int
}

// ColorIndex is the swatch a Project name picks; stable across restarts and
// themes because it is a hash of the name alone.
func ColorIndex(name string, swatches int) int {
	if swatches <= 0 {
		return 0
	}
	h := fnv.New32a()
	h.Write([]byte(name))
	return int(h.Sum32() % uint32(swatches))
}

type painter struct {
	palette Palette
	basic   bool
}

func (p painter) fg(hex string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(hex))
}

func (p painter) status(status string) string {
	switch status {
	case runs.StatusRunning:
		return p.palette.Working
	case runs.StatusNeedsHuman:
		return p.palette.Waiting
	case runs.StatusDone:
		return p.palette.Finished
	case runs.StatusFailed:
		return p.palette.Errored
	}
	return p.palette.Idle
}

// Draw renders the Runs Overview at width. windowKey is the key that steps the
// window, named on the label. Every line is at most width cells wide.
func Draw(o Runs, width int, windowKey string, mode presentation.GlyphMode, palette Palette) Drawing {
	p := painter{palette: palette, basic: mode == presentation.GlyphModeBasic}
	inner := max(width-2, 10)
	label := p.fg(palette.Dim).Render("Window ") + p.fg(palette.Bright).Bold(true).Render(o.Window.Label())
	hint := "click"
	if windowKey != "" {
		hint = windowKey + " or click"
	}
	label += p.fg(palette.Dim).Render("  " + hint + " for " + o.Window.Next().Label())
	d := Drawing{Lines: []string{fit(label, inner), ""}}
	if len(o.Projects) == 0 {
		empty := "No Runs yet"
		if o.Window != WindowAll {
			empty = "No Runs in the last " + o.Window.Label()
		}
		d.Lines = append(d.Lines, p.fg(palette.Dim).Render(empty))
		return d
	}
	for _, project := range o.Projects {
		d.Lines = append(d.Lines, p.block(project, inner)...)
		d.Lines = append(d.Lines, "")
	}
	return d
}

func fit(s string, width int) string { return ansi.Truncate(s, width, "…") }

func (p painter) block(project Project, width int) []string {
	swatches := p.palette.Swatches
	color := p.palette.Bright
	if len(swatches) > 0 {
		color = swatches[ColorIndex(project.Name, len(swatches))]
	}
	body := width - 4
	var rows []string
	header := p.fg(color).Bold(true).Render(project.Name) + "  " + p.badge(project.Mode)
	rows = append(rows, wrap(header, body)...)
	var facts []string
	if project.Base != "" {
		facts = append(facts, "base "+project.Base)
	}
	facts = append(facts, "Ticket Gate "+project.TicketGate, gate(project.Merge))
	rows = append(rows, wrap(p.fg(p.palette.Dim).Render(strings.Join(facts, " · ")), body)...)
	rows = append(rows, "")
	rows = append(rows, p.inFlight(project, body))
	rows = append(rows, p.throughput(project, body)...)
	rows = append(rows, p.funnel(project.Funnel, body)...)
	rows = append(rows, p.durations(project.Durations, body))
	rows = append(rows, p.latest(project.Latest, body)...)
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(color)).Padding(0, 1).Width(width - 2)
	return strings.Split(box.Render(strings.Join(rows, "\n")), "\n")
}

func gate(merge string) string {
	switch {
	case strings.HasPrefix(merge, "PR Gate"):
		return "PR Gate on · auto-merge off"
	case strings.HasPrefix(merge, "Auto-merge"):
		return "PR Gate off · auto-merge on"
	}
	return "PR Gate " + runs.Unknown + " · auto-merge " + runs.Unknown
}

func (p painter) badge(mode string) string {
	return p.fg(p.palette.Bright).Background(lipgloss.Color(p.palette.Dim)).Render(" " + strings.ToUpper(mode) + " ")
}

func wrap(s string, width int) []string {
	return strings.Split(ansi.Wrap(s, width, ""), "\n")
}

func (p painter) label(s string) string { return p.fg(p.palette.Dim).Render(s) }

func (p painter) cell() string {
	if p.basic {
		return "#"
	}
	return "█"
}

func (p painter) inFlight(project Project, width int) string {
	total := project.Queued + project.Running + project.NeedsHuman
	if total == 0 {
		return p.label("In flight  ") + "nothing"
	}
	strip := p.fg(p.palette.Idle).Render(strings.Repeat(p.cell(), project.Queued)) +
		p.fg(p.palette.Working).Render(strings.Repeat(p.cell(), project.Running)) +
		p.fg(p.palette.Waiting).Render(strings.Repeat(p.cell(), project.NeedsHuman))
	counts := fmt.Sprintf("%d queued · %d running · %d needs-human", project.Queued, project.Running, project.NeedsHuman)
	line := p.label("In flight  ") + strip + " " + counts
	if ansi.StringWidth(line) > width {
		// The counts still say it when a long strip would crowd them out.
		line = p.label("In flight  ") + counts
	}
	return line
}

// merge folds buckets into groups of factor, so a long window fits the width.
func merge(buckets []Bucket, hourly bool, fit int) ([]Bucket, int) {
	factor := 1
	for (len(buckets)+factor-1)/factor > fit {
		switch {
		case hourly:
			factor++
		case factor == 1:
			factor = 7
		default:
			factor += 7
		}
	}
	if factor == 1 {
		return buckets, 1
	}
	var out []Bucket
	for i := 0; i < len(buckets); i += factor {
		group := Bucket{Start: buckets[i].Start}
		for _, b := range buckets[i:min(i+factor, len(buckets))] {
			group.Done += b.Done
			group.Failed += b.Failed
			group.NeedsHuman += b.NeedsHuman
		}
		out = append(out, group)
	}
	return out, factor
}

func (p painter) throughput(project Project, width int) []string {
	var done, failed, needs int
	for _, b := range project.Buckets {
		done, failed, needs = done+b.Done, failed+b.Failed, needs+b.NeedsHuman
	}
	if done+failed+needs == 0 {
		return []string{p.label("Finished   ") + "nothing finished"}
	}
	bars, factor := merge(project.Buckets, project.Hourly, width)
	unit := "day"
	switch {
	case project.Hourly && factor == 1:
		unit = "hour"
	case project.Hourly:
		unit = fmt.Sprintf("%d hours", factor)
	case factor == 7:
		unit = "week"
	case factor > 1:
		unit = fmt.Sprintf("%d days", factor)
	}
	legend := fmt.Sprintf("%s %d done  %s %d failed  %s %d needs-human",
		p.fg(p.palette.Finished).Render(p.cell()), done, p.fg(p.palette.Errored).Render(p.cell()), failed, p.fg(p.palette.Waiting).Render(p.cell()), needs)
	rows := []string{p.label("Finished per " + unit)}
	rows = append(rows, wrap(legend, width)...)
	chart := barchart.New(len(bars), 4, barchart.WithNoAxis(), barchart.WithNoAutoBarWidth(), barchart.WithBarWidth(1), barchart.WithBarGap(0))
	for _, b := range bars {
		chart.Push(barchart.BarData{Values: []barchart.BarValue{
			{Name: "done", Value: float64(b.Done), Style: p.fg(p.palette.Finished)},
			{Name: "failed", Value: float64(b.Failed), Style: p.fg(p.palette.Errored)},
			{Name: "needs-human", Value: float64(b.NeedsHuman), Style: p.fg(p.palette.Waiting)},
		}})
	}
	chart.Draw()
	return append(rows, strings.Split(chart.View(), "\n")...)
}

func (p painter) funnel(f Funnel, width int) []string {
	rows := []string{p.label("Pull requests, counted in Tickets")}
	stages := []struct {
		name  string
		n     int
		color string
	}{{"Tickets", f.Tickets, p.palette.Text}, {"opened", f.Opened, p.palette.Working}, {"merged", f.Merged, p.palette.Finished}, {"closed", f.Closed, p.palette.Errored}}
	room := max(width-14, 1)
	for _, s := range stages {
		length := 0
		if f.Tickets > 0 {
			length = (s.n*room + f.Tickets - 1) / f.Tickets
		}
		rows = append(rows, fmt.Sprintf("%-8s %3d ", s.name, s.n)+p.fg(s.color).Render(strings.Repeat(p.cell(), length)))
	}
	return rows
}

func (p painter) durations(list []time.Duration, width int) string {
	if len(list) == 0 {
		return p.label("Run time  ") + "no finished Run with a recorded end"
	}
	const prefix = "Run time  "
	suffix := fmt.Sprintf("  latest %s · longest %s", span(list[len(list)-1]), span(maxOf(list)))
	room := max(width-len(prefix)-ansi.StringWidth(suffix), 4)
	if len(list) > room {
		list = list[len(list)-room:]
	}
	chart := sparkline.New(len(list), 1, sparkline.WithStyle(p.fg(p.palette.Working)))
	for _, d := range list {
		chart.Push(d.Seconds())
	}
	if p.basic {
		chart.Draw()
	} else {
		chart.DrawBraille()
	}
	return p.label(prefix) + chart.View() + p.label(suffix)
}

func maxOf(list []time.Duration) time.Duration {
	out := list[0]
	for _, d := range list {
		out = max(out, d)
	}
	return out
}

// span is a duration to one or two units: 45s, 12m, 1h 5m, 3d 4h.
func span(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d/time.Second))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", int(d/time.Hour), int(d%time.Hour/time.Minute))
	}
	return fmt.Sprintf("%dd %dh", int(d/(24*time.Hour)), int(d%(24*time.Hour)/time.Hour))
}

func (p painter) latest(list []Finished, width int) []string {
	if len(list) == 0 {
		return []string{p.label("Latest     ") + "no finished Runs"}
	}
	rows := []string{p.label("Latest finished")}
	for _, f := range list {
		rows = append(rows, wrap(p.fg(p.status(f.Status)).Render(p.cell())+" "+p.fg(p.palette.Bright).Render(f.Title), width)...)
		summary := f.Summary
		if summary == "" {
			summary = "no summary recorded"
		}
		for _, line := range wrap(p.fg(p.palette.Dim).Render(summary), width-2) {
			rows = append(rows, "  "+line)
		}
	}
	return rows
}
