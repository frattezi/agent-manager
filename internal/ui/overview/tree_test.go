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
		{"a lone unfinished Ticket is its own root", []runs.Record{run1("o/api#1", runs.StatusRunning, nil)}, nil, "o/api#1"},
		{"a lone finished Ticket is not", []runs.Record{run1("o/api#1", runs.StatusDone, nil)}, nil, ""},
		{"held blockers win over the Run's", []runs.Record{
			run1("o/api#1", runs.StatusRunning, nil),
			run1("o/api#4", runs.StatusRunning, nil),
			run1("o/api#2", runs.StatusQueued, blocked("o/api#1")),
		}, []runs.HeldTicket{{RepoPath: "/src/api", TicketRef: "o/api#2", Blockers: []string{"o/api#4"}, Holds: []runs.Hold{{Blocker: "o/api#4", Reason: "not done"}}}},
			"o/api#1,o/api#4(o/api#2)"},
		{"a held Ticket of another Project is ignored", []runs.Record{
			run1("o/api#2", runs.StatusQueued, nil),
		}, []runs.HeldTicket{{RepoPath: "/src/web", TicketRef: "o/api#2", Blockers: []string{"o/api#4"}}}, "o/api#2"},
		{"a cycle is drawn once, as roots", []runs.Record{
			run1("o/api#1", runs.StatusQueued, blocked("o/api#2")),
			run1("o/api#2", runs.StatusQueued, blocked("o/api#1")),
			run1("o/api#3", runs.StatusQueued, blocked("o/api#2")),
		}, nil, "o/api#1@,o/api#2@(o/api#3)"},
		{"a finished tree outside the window is dropped", []runs.Record{
			run("1", "o/api#1", "api", runs.StatusDone, 40*24*time.Hour, nil),
			run("2", "o/api#2", "api", runs.StatusDone, 40*24*time.Hour, blocked("o/api#1")),
			run("3", "o/api#8", "api", runs.StatusRunning, time.Hour, nil),
		}, nil, "o/api#8"},
		{"an unfinished tree outside the window is kept", []runs.Record{
			run("1", "o/api#1", "api", runs.StatusDone, 40*24*time.Hour, nil),
			run("2", "o/api#2", "api", runs.StatusQueued, 40*24*time.Hour, blocked("o/api#1")),
			run("3", "o/api#8", "api", runs.StatusRunning, time.Hour, nil),
		}, nil, "o/api#1(o/api#2),o/api#8"},
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
	held := []runs.HeldTicket{{RepoPath: "/src/api", TicketRef: "o/api#3", Blockers: []string{"o/api#1", "o/api#2"},
		Holds: []runs.Hold{{Blocker: "o/api#2", Reason: "not done: its latest Run failed"}}}}
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

func progressSample() Runs {
	list := []runs.Record{
		run("1", "o/api#1", "api", runs.StatusDone, 3*time.Hour, func(r *runs.Record) {
			r.CreatedAt, r.FinishedAt = now.Add(-3*time.Hour), now.Add(-3*time.Hour+20*time.Minute)
		}),
		run("2", "o/api#2", "api", runs.StatusDone, 2*time.Hour, func(r *runs.Record) {
			r.CreatedAt, r.FinishedAt = now.Add(-2*time.Hour), now.Add(-2*time.Hour+24*time.Minute)
		}),
		run("3", "o/api#3", "api", runs.StatusRunning, time.Minute, func(r *runs.Record) {
			r.Title, r.CreatedAt, r.LogTail = "Seed the data", now.Add(-11*time.Minute), "writing seed.sql"
		}),
		run("4", "o/api#4", "api", runs.StatusNeedsHuman, time.Hour, func(r *runs.Record) {
			r.Title, r.Result = "Pick a store", &runs.Result{Question: "Postgres or SQLite?\nThe spec says both."}
		}),
	}
	return Compute(list, WindowAll, now, nil)
}

func TestUsualRunTimeIsTheMedianOfFinishedRuns(t *testing.T) {
	if got := progressSample().Projects[0].Usual; got != 22*time.Minute {
		t.Errorf("Usual = %v, want 22m", got)
	}
}

func TestDrawRunningTicketShowsProgressAndLogLine(t *testing.T) {
	for _, mode := range []presentation.GlyphMode{presentation.GlyphModeAdvanced, presentation.GlyphModeBasic} {
		for _, width := range []int{34, 100} {
			text := ansi.Strip(strings.Join(Draw(progressSample(), width, "W", mode, testPalette).Lines, "\n"))
			flat := strings.Join(strings.Fields(strings.ReplaceAll(text, "│", " ")), " ")
			if width == 100 {
				for _, want := range []string{"11m of a usual 22m", "writing seed.sql", "Postgres or SQLite?"} {
					if !strings.Contains(flat, want) {
						t.Errorf("mode %v width %d: lacks %q:\n%s", mode, width, want, text)
					}
				}
				if strings.Contains(flat, "The spec says both") {
					t.Errorf("question is more than its first line:\n%s", text)
				}
			}
			if tree, charts := strings.Index(text, "Blocker Tree"), strings.Index(text, "Finished per day"); tree < 0 || charts >= 0 && tree > charts {
				t.Errorf("mode %v width %d: tree is not before the charts:\n%s", mode, width, text)
			}
			if mode == presentation.GlyphModeBasic && strings.ContainsAny(text[strings.Index(text, "Blocker Tree"):strings.Index(text, "Finished per day")], "█░├└") {
				t.Errorf("non-ASCII glyph under Compatible mode:\n%s", text)
			}
			for _, line := range strings.Split(text, "\n") {
				if w := ansi.StringWidth(line); w > width {
					t.Errorf("width %d: line is %d cells: %q", width, w, line)
				}
			}
		}
	}
}

func TestDrawRunningTicketWithNoFinishedRunsShowsOnlyItsTime(t *testing.T) {
	list := []runs.Record{run("3", "o/api#3", "api", runs.StatusRunning, time.Minute, func(r *runs.Record) { r.CreatedAt = now.Add(-13 * time.Minute) })}
	text := ansi.Strip(strings.Join(Draw(Compute(list, WindowAll, now, nil), 100, "W", presentation.GlyphModeAdvanced, testPalette).Lines, "\n"))
	if !strings.Contains(text, "13m") || strings.Contains(text, "usual") {
		t.Errorf("want the elapsed time alone:\n%s", text)
	}
}
