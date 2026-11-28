package overview

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	"github.com/YoanWai/agent-manager/internal/ui/runs"
)

func blocked(by ...string) func(*runs.Record) {
	return func(r *runs.Record) { r.BlockedBy = by }
}

func shape(nodes []*Node) string {
	var b strings.Builder
	for i, n := range nodes {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(n.Ref)
		if n.Join {
			b.WriteString("~")
		}
		if n.Cycle {
			b.WriteString("@")
		}
		if len(n.Children) > 0 {
			b.WriteString("(" + shape(n.Children) + ")")
		}
	}
	return b.String()
}

func tree(t *testing.T, list []runs.Record, held []runs.HeldTicket, window Window) []*Node {
	t.Helper()
	o := Compute(list, window, now, held)
	if len(o.Projects) == 0 {
		return nil
	}
	return o.Projects[0].Tree
}

func TestBlockerTreeShapes(t *testing.T) {
	run1 := func(ref string, status string, edit func(*runs.Record)) runs.Record {
		return run(ref, ref, "api", status, time.Hour, edit)
	}
	for _, tc := range []struct {
		name string
		list []runs.Record
		held []runs.HeldTicket
		want string
	}{
		{"a chain", []runs.Record{
			run1("o/api#1", runs.StatusDone, nil),
			run1("o/api#2", runs.StatusRunning, blocked("o/api#1")),
			run1("o/api#3", runs.StatusQueued, blocked("o/api#2")),
		}, nil, "o/api#1(o/api#2(o/api#3))"},
		{"two blockers, second copy a join", []runs.Record{
			run1("o/api#1", runs.StatusRunning, nil),
			run1("o/api#2", runs.StatusRunning, nil),
			run1("o/api#3", runs.StatusQueued, blocked("o/api#1", "o/api#2")),
		}, nil, "o/api#1(o/api#3),o/api#2(o/api#3~)"},
		{"a blocker with no Run", []runs.Record{
			run1("o/api#2", runs.StatusQueued, blocked("o/api#9")),
		}, nil, "o/api#9(o/api#2)"},
		{"no blockers anywhere", []runs.Record{run1("o/api#1", runs.StatusRunning, nil)}, nil, ""},
		{"held blockers win over the Run's", []runs.Record{
			run1("o/api#1", runs.StatusRunning, nil),
			run1("o/api#4", runs.StatusRunning, nil),
			run1("o/api#2", runs.StatusQueued, blocked("o/api#1")),
		}, []runs.HeldTicket{{Repo: "/src/api", TicketRef: "o/api#2", BlockedBy: []runs.Blocker{{Ref: "o/api#4", Reason: "not done"}}}},
			"o/api#4(o/api#2)"},
		{"a held Ticket of another Project is ignored", []runs.Record{
			run1("o/api#2", runs.StatusQueued, nil),
		}, []runs.HeldTicket{{Repo: "/src/web", TicketRef: "o/api#2", BlockedBy: []runs.Blocker{{Ref: "o/api#4"}}}}, ""},
		{"a cycle is drawn once, as roots", []runs.Record{
			run1("o/api#1", runs.StatusQueued, blocked("o/api#2")),
			run1("o/api#2", runs.StatusQueued, blocked("o/api#1")),
			run1("o/api#3", runs.StatusQueued, blocked("o/api#2")),
		}, nil, "o/api#1@,o/api#2@(o/api#3)"},
		{"a finished tree outside the window is dropped", []runs.Record{
			run("1", "o/api#1", "api", runs.StatusDone, 40*24*time.Hour, nil),
			run("2", "o/api#2", "api", runs.StatusDone, 40*24*time.Hour, blocked("o/api#1")),
			run("3", "o/api#8", "api", runs.StatusRunning, time.Hour, nil),
		}, nil, ""},
		{"an unfinished tree outside the window is kept", []runs.Record{
			run("1", "o/api#1", "api", runs.StatusDone, 40*24*time.Hour, nil),
			run("2", "o/api#2", "api", runs.StatusQueued, 40*24*time.Hour, blocked("o/api#1")),
			run("3", "o/api#8", "api", runs.StatusRunning, time.Hour, nil),
		}, nil, "o/api#1(o/api#2)"},
	} {
		window := WindowAll
		if strings.Contains(tc.name, "window") {
			window = Window7d
		}
		if got := shape(tree(t, tc.list, tc.held, window)); got != tc.want {
			t.Errorf("%s: tree = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func treeSample() Runs {
	list := []runs.Record{
		run("1", "o/api#1", "api", runs.StatusDone, time.Hour, func(r *runs.Record) { r.Title = "Add the schema" }),
		run("2", "o/api#2", "api", runs.StatusFailed, time.Hour, func(r *runs.Record) { r.Title = "Add the endpoint" }),
		run("3", "o/api#3", "api", runs.StatusQueued, time.Hour, func(r *runs.Record) { r.Title = "Wire the client"; r.BlockedBy = []string{"o/api#1", "o/api#2"} }),
		run("5", "o/api#5", "api", runs.StatusRunning, time.Hour, func(r *runs.Record) { r.Title = "Seed the data"; r.BlockedBy = []string{"o/api#1"} }),
		run("4", "o/api#4", "api", runs.StatusQueued, time.Hour, func(r *runs.Record) { r.Title = "Document it"; r.BlockedBy = []string{"o/api#9"} }),
	}
	held := []runs.HeldTicket{{Repo: "/src/api", TicketRef: "o/api#3", BlockedBy: []runs.Blocker{
		{Ref: "o/api#1"}, {Ref: "o/api#2", Reason: "not done: its latest Run failed"}}}}
	return Compute(list, WindowAll, now, held)
}

func TestDrawBlockerTreeAtNarrowAndWideWidths(t *testing.T) {
	for _, width := range []int{34, 100} {
		d := Draw(treeSample(), width, "W", presentation.GlyphModeAdvanced, testPalette)
		text := ansi.Strip(strings.Join(d.Lines, "\n"))
		flat := strings.Join(strings.Fields(strings.NewReplacer("│", " ", "└─", " ", "├─", " ").Replace(text)), " ")
		for _, want := range []string{"Blocker Tree", "Add the schema #1", "Add the endpoint #2", "Wire the client #3", "(join)", "#9 no Run yet"} {
			if !strings.Contains(flat, want) && width == 100 {
				t.Errorf("width %d: lacks %q:\n%s", width, want, text)
			}
		}
		if width == 100 && !strings.Contains(flat, "not done: its latest Run failed") {
			t.Errorf("hold reason missing:\n%s", text)
		}
		if !strings.Contains(text, "├─") || !strings.Contains(text, "└─") {
			t.Errorf("width %d: no box-drawing edges:\n%s", width, text)
		}
		for _, line := range d.Lines {
			if w := ansi.StringWidth(line); w > width {
				t.Errorf("width %d: line is %d cells: %q", width, w, ansi.Strip(line))
			}
		}
	}
}

func TestDrawBlockerTreeUsesASCIIUnderTheCompatibleGlyphMode(t *testing.T) {
	d := Draw(treeSample(), 80, "W", presentation.GlyphModeBasic, testPalette)
	text := ansi.Strip(strings.Join(d.Lines, "\n"))
	if !strings.Contains(text, "`- ") || !strings.Contains(text, "|- ") {
		t.Errorf("no ASCII edges:\n%s", text)
	}
	tree := text[strings.Index(text, "Blocker Tree"):]
	if end := strings.Index(tree, "╰"); end > 0 {
		tree = tree[:end]
	}
	for _, r := range "├└┬" {
		if strings.ContainsRune(tree, r) {
			t.Errorf("box-drawing %q in the tree under Compatible mode:\n%s", r, tree)
		}
	}
}
