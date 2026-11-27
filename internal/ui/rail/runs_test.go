package rail_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/keybind"
	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	"github.com/YoanWai/agent-manager/internal/ui/rail"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func runsSnapshot() rail.Snapshot {
	return rail.Snapshot{
		Sessions: []rail.Session{{ID: "one", Name: "one"}},
		Runs: []rail.Run{
			{ID: "r1", Ticket: "jira:ABC-1", Status: "needs-human", Branch: "fractory/abc-1"},
			{ID: "r2", Ticket: "github:o/r#2", Status: "done", Branch: "fractory/r-2"},
		},
	}
}

func TestRunsSectionFollowsTheSessions(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(runsSnapshot())
	var got []rail.RowKind
	for _, row := range model.Rows() {
		got = append(got, row.Kind)
	}
	want := []rail.RowKind{rail.GroupRow, rail.SessionRow, rail.RunsRow, rail.RunRow, rail.RunRow, rail.BotsRow}
	if len(got) != len(want) {
		t.Fatalf("kinds = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("kinds = %v, want %v", got, want)
		}
	}
}

func TestNoRunsNoSection(t *testing.T) {
	model := rail.New(nil)
	snapshot := runsSnapshot()
	snapshot.Runs = nil
	model.Reconcile(snapshot)
	for _, row := range model.Rows() {
		if row.Kind == rail.RunsRow || row.Kind == rail.RunRow {
			t.Fatalf("listed %+v with no Runs", row)
		}
	}
}

func TestEnterOnARunAsksForItsLog(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(runsSnapshot())
	model.Focus(rail.Selection{Kind: rail.RunRow, Run: "r1"})
	decision := model.Key(keyFor("enter"), rail.Frame{}, rail.KeyContext{ListKeys: keybind.DefaultList()})
	target := decision.Intent.Target
	if decision.Intent.Kind != rail.Attach || target.Kind != rail.RunRow || target.Run != "r1" {
		t.Fatalf("intent = %+v", decision.Intent)
	}
}

func TestARunCannotBeReordered(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(runsSnapshot())
	model.Focus(rail.Selection{Kind: rail.RunRow, Run: "r1"})
	decision := model.Key(keyFor("shift+up"), rail.Frame{}, rail.KeyContext{ListKeys: keybind.DefaultList()})
	if len(decision.Mutations) != 0 || decision.Error != "Runs are read-only" {
		t.Fatalf("reordering a run: %+v", decision)
	}
}

func TestSearchNarrowsRunsAndKeepsOtherRowsOut(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(runsSnapshot())
	model.SetSearch("abc-1", false)
	var runs []string
	for _, row := range model.Rows() {
		if row.Kind == rail.RunRow {
			runs = append(runs, row.Name)
		}
	}
	if len(runs) != 1 || runs[0] != "jira:ABC-1" {
		t.Fatalf("runs after search = %v", runs)
	}
}

func TestRunRowsPaintTicketAndStatus(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(runsSnapshot())
	frame := model.Render(renderContext(48, 8), rail.Frame{})
	var text string
	for _, line := range frame.Lines {
		text += ansi.Strip(line.Text) + "\n"
	}
	for _, want := range []string{"Runs", "jira:ABC-1", "needs-human", "github:o/r#2", "done"} {
		if !strings.Contains(text, want) {
			t.Fatalf("frame lacks %q:\n%s", want, text)
		}
	}
}

func TestRunRowsPaintTheModeBadge(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(rail.Snapshot{Runs: []rail.Run{
		{ID: "w", Ticket: "jira:W-1", Status: "done", Mode: "work"},
		{ID: "p", Ticket: "jira:P-1", Status: "done", Mode: "personal"},
		{ID: "u", Ticket: "jira:U-1", Status: "done"},
	}})
	frame := model.Render(renderContext(48, 8), rail.Frame{})
	got := map[string]string{}
	for _, line := range frame.Lines {
		text := ansi.Strip(line.Text)
		for _, ticket := range []string{"W-1", "P-1", "U-1"} {
			if strings.Contains(text, ticket) {
				got[ticket] = strings.TrimSpace(text)
			}
		}
	}
	for ticket, badge := range map[string]string{"W-1": "W done", "P-1": "P done"} {
		if !strings.Contains(got[ticket], badge) {
			t.Fatalf("row %s = %q; want %q", ticket, got[ticket], badge)
		}
	}
	if strings.Contains(got["U-1"], " done") && strings.ContainsAny(strings.Fields(got["U-1"])[len(strings.Fields(got["U-1"]))-2], "WP") {
		t.Fatalf("row without a mode shows a badge: %q", got["U-1"])
	}
}

func TestRunRowsPaintTheRerunMarkerPerGlyphMode(t *testing.T) {
	snapshot := rail.Snapshot{Runs: []rail.Run{
		{ID: "a", Ticket: "jira:A-1", Status: "failed", Reruns: 2},
		{ID: "b", Ticket: "jira:B-1", Status: "failed", RerunPending: true},
		{ID: "c", Ticket: "jira:C-1", Status: "failed"},
	}}
	rowFor := func(mode presentation.GlyphMode, ticket string) string {
		model := rail.New(nil)
		model.Reconcile(snapshot)
		ctx := renderContext(48, 8)
		ctx.GlyphMode = mode
		for _, line := range model.Render(ctx, rail.Frame{}).Lines {
			if text := ansi.Strip(line.Text); strings.Contains(text, ticket) {
				return text
			}
		}
		t.Fatalf("no row for %s", ticket)
		return ""
	}
	for mode, want := range map[presentation.GlyphMode][2]string{
		presentation.GlyphModeAdvanced: {"↻2", "↻…"},
		presentation.GlyphModeBasic:    {"R2", "R…"},
	} {
		if got := rowFor(mode, "A-1"); !strings.Contains(got, want[0]) {
			t.Errorf("%s: row %q lacks %q", mode, got, want[0])
		}
		if got := rowFor(mode, "B-1"); !strings.Contains(got, want[1]) {
			t.Errorf("%s: row %q lacks %q", mode, got, want[1])
		}
		if got := rowFor(mode, "C-1"); strings.Contains(got, "↻") || strings.Contains(got, " R") {
			t.Errorf("%s: row without Re-runs shows a marker: %q", mode, got)
		}
	}
}

// headerLine is the Runs header's text after its fold mark.
func headerLine(t *testing.T, model rail.Model) string {
	t.Helper()
	_, text := headerParts(t, model, renderContext(48, 8))
	return text
}

func headerParts(t *testing.T, model rail.Model, ctx rail.RenderContext) (mark, text string) {
	t.Helper()
	for _, line := range model.Render(ctx, rail.Frame{}).Lines {
		if line.HitOK && line.Hit.Kind == rail.RunsRow {
			mark, text, _ = strings.Cut(strings.TrimSpace(ansi.Strip(line.Text)), " ")
			return mark, text
		}
	}
	t.Fatal("no Runs header")
	return "", ""
}

func TestRunsHeaderCountsListedAndFinishedTickets(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(runsSnapshot())
	if got := headerLine(t, model); !strings.HasPrefix(got, "Runs (2) fractory") {
		t.Fatalf("header = %q; want the listed count alone", got)
	}
	snapshot := runsSnapshot()
	snapshot.RunsFinished = 9
	model.Reconcile(snapshot)
	if got := headerLine(t, model); !strings.HasPrefix(got, "Runs (2, 9 finished) fractory") {
		t.Fatalf("header = %q; want the finished count too", got)
	}
}

func TestRunsHeaderStaysWhenEveryTicketIsFinished(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(rail.Snapshot{RunsFinished: 3})
	if got := headerLine(t, model); !strings.HasPrefix(got, "Runs (0, 3 finished)") {
		t.Fatalf("header = %q", got)
	}
}

func TestMarkHandledKeyOnARunAsksToToggleSolved(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(runsSnapshot())
	model.Focus(rail.Selection{Kind: rail.RunRow, Run: "r1"})
	decision := model.Key(keyFor("."), rail.Frame{}, rail.KeyContext{ListKeys: keybind.DefaultList()})
	if target := decision.Intent.Target; decision.Intent.Kind != rail.MarkIdle || target.Kind != rail.RunRow || target.Run != "r1" {
		t.Fatalf("intent = %+v", decision.Intent)
	}
}

func TestFinishedRunsKeyAsksForTheToggleOnlyInRuns(t *testing.T) {
	ctx := rail.KeyContext{ListKeys: keybind.DefaultList()}
	model := rail.New(nil)
	snapshot := runsSnapshot()
	snapshot.RunsFinished = 1
	model.Reconcile(snapshot)
	for _, selection := range []rail.Selection{{Kind: rail.RunRow, Run: "r1"}, {Kind: rail.RunsRow}} {
		model.Focus(selection)
		if decision := model.Key(keyFor("H"), rail.Frame{}, ctx); decision.Intent.Kind != rail.ToggleFinishedRuns {
			t.Fatalf("H on %+v: intent = %+v", selection, decision.Intent)
		}
	}
	model.Focus(rail.Selection{Kind: rail.SessionRow, SessionID: "one"})
	if decision := model.Key(keyFor("H"), rail.Frame{}, ctx); decision.Intent.Kind != rail.NoAction {
		t.Fatalf("H on a session: intent = %+v", decision.Intent)
	}
	model.Reconcile(runsSnapshot())
	model.Focus(rail.Selection{Kind: rail.RunsRow})
	if decision := model.Key(keyFor("H"), rail.Frame{}, ctx); decision.Intent.Kind != rail.NoAction {
		t.Fatalf("H with nothing finished: intent = %+v", decision.Intent)
	}
}

func TestFinishedRunsActionFollowsTheToggle(t *testing.T) {
	model := rail.New(nil)
	snapshot := runsSnapshot()
	model.Reconcile(snapshot)
	if got := model.FinishedRunsAction(); got != "" {
		t.Fatalf("nothing finished: %q", got)
	}
	snapshot.RunsFinished = 2
	model.Reconcile(snapshot)
	if got := model.FinishedRunsAction(); got != "show finished" {
		t.Fatalf("hidden: %q", got)
	}
	snapshot.RunsShowFinished = true
	model.Reconcile(snapshot)
	if got := model.FinishedRunsAction(); got != "hide finished" {
		t.Fatalf("shown: %q", got)
	}
}

func runKinds(model rail.Model) []rail.RowKind {
	var kinds []rail.RowKind
	for _, row := range model.Rows() {
		if row.Kind == rail.RunsRow || row.Kind == rail.RunRow {
			kinds = append(kinds, row.Kind)
		}
	}
	return kinds
}

func runsFolded(t *testing.T, model rail.Model) bool {
	t.Helper()
	switch kinds := runKinds(model); len(kinds) {
	case 1:
		return true
	case 3:
		return false
	default:
		t.Fatalf("Runs rows = %v", kinds)
	}
	return false
}

func TestRunsHeaderFoldsLikeAGroupHeader(t *testing.T) {
	ctx := rail.KeyContext{ListKeys: keybind.DefaultList(), ArrowStep: true}
	model := rail.New(nil)
	model.Reconcile(runsSnapshot())
	model.Focus(rail.Selection{Kind: rail.RunsRow})
	decision := model.Key(keyFor("enter"), rail.Frame{}, ctx)
	if !runsFolded(t, model) || decision.Intent.Kind != rail.NoAction {
		t.Fatalf("enter did not fold: intent %+v", decision.Intent)
	}
	if len(decision.Mutations) != 1 || decision.Mutations[0].Kind != rail.SaveCollapsed ||
		!slices.Contains(decision.Mutations[0].HostCollapsed, rail.RunsFold) || len(decision.Mutations[0].Collapsed) != 0 {
		t.Fatalf("fold save = %+v", decision.Mutations)
	}
	if !model.SelectionCollapsed(rail.Selection{Kind: rail.RunsRow}) {
		t.Fatal("folded header does not report folded")
	}
	model.Key(keyFor("left"), rail.Frame{}, ctx)
	if !runsFolded(t, model) {
		t.Fatal("step out unfolded a folded header")
	}
	model.Key(keyFor("right"), rail.Frame{}, ctx)
	if runsFolded(t, model) {
		t.Fatal("step in did not unfold")
	}
	model.Key(keyFor("right"), rail.Frame{}, ctx)
	if runsFolded(t, model) {
		t.Fatal("step in folded an open header")
	}
	model.Key(keyFor("left"), rail.Frame{}, ctx)
	if !runsFolded(t, model) {
		t.Fatal("step out did not fold")
	}
	model.Key(keyFor("enter"), rail.Frame{}, ctx)
	if runsFolded(t, model) {
		t.Fatal("enter did not unfold")
	}
}

func TestFoldAllFoldsTheRunsSectionWithTheGroups(t *testing.T) {
	ctx := rail.KeyContext{ListKeys: keybind.DefaultList()}
	snapshot := runsSnapshot()
	snapshot.Groups = []string{"work"}
	snapshot.Sessions = append(snapshot.Sessions, rail.Session{ID: "two", Name: "two", Group: "work"})
	model := rail.New(nil)
	model.Reconcile(snapshot)
	model.Key(keyFor("F"), rail.Frame{}, ctx)
	if !runsFolded(t, model) || !model.IsCollapsed("work") || !model.AllGroupsCollapsed() {
		t.Fatalf("fold all left Runs %v, work folded %v", runKinds(model), model.IsCollapsed("work"))
	}
	model.Key(keyFor("F"), rail.Frame{}, ctx)
	if runsFolded(t, model) || model.IsCollapsed("work") {
		t.Fatal("unfold all left something folded")
	}
	// An open Runs section alone keeps fold all from reading as done.
	model.Focus(rail.Selection{Kind: rail.GroupRow, Group: "work"})
	model.Key(keyFor("enter"), rail.Frame{}, ctx)
	if model.AllGroupsCollapsed() {
		t.Fatal("every group folded but Runs open reads as all folded")
	}
}

func TestFoldedRunsHeaderKeepsItsCountsAndTheFinishedKey(t *testing.T) {
	ctx := rail.KeyContext{ListKeys: keybind.DefaultList()}
	snapshot := runsSnapshot()
	snapshot.RunsFinished = 9
	model := rail.New(nil)
	model.RestoreHostFolds([]rail.HostFold{rail.RunsFold})
	model.Reconcile(snapshot)
	if !runsFolded(t, model) {
		t.Fatal("a restored fold did not fold Runs")
	}
	if got := headerLine(t, model); !strings.HasPrefix(got, "Runs (2, 9 finished) fractory") {
		t.Fatalf("folded header = %q", got)
	}
	model.Focus(rail.Selection{Kind: rail.RunsRow})
	if decision := model.Key(keyFor("H"), rail.Frame{}, ctx); decision.Intent.Kind != rail.ToggleFinishedRuns {
		t.Fatalf("H on a folded header: intent = %+v", decision.Intent)
	}
	if !runsFolded(t, model) {
		t.Fatal("H unfolded the header")
	}
}

func TestSearchShowsTheRunsAFoldHides(t *testing.T) {
	model := rail.New(nil)
	model.RestoreHostFolds([]rail.HostFold{rail.RunsFold})
	model.Reconcile(runsSnapshot())
	model.SetSearch("abc", false)
	if kinds := runKinds(model); len(kinds) != 2 {
		t.Fatalf("search under a fold listed %v; want the header and the match", kinds)
	}
}

func TestRunsFoldIsANoOpInTheArchivedView(t *testing.T) {
	ctx := rail.KeyContext{ListKeys: keybind.DefaultList()}
	model := rail.New(nil)
	model.Reconcile(runsSnapshot())
	model.SetArchived(true)
	decision := model.Key(keyFor("F"), rail.Frame{}, ctx)
	model.SetArchived(false)
	if len(decision.Mutations) != 0 || runsFolded(t, model) {
		t.Fatalf("archived fold all folded Runs: %+v", decision)
	}
}

func TestRunsHeaderPaintsTheGroupFoldMarks(t *testing.T) {
	for _, mode := range []presentation.GlyphMode{presentation.GlyphModeBasic, presentation.GlyphModeAdvanced} {
		glyphs := presentation.GlyphsFor(mode)
		ctx := renderContext(48, 8)
		ctx.GlyphMode = mode
		model := rail.New(nil)
		model.Reconcile(runsSnapshot())
		if mark, _ := headerParts(t, model, ctx); mark != glyphs.GroupOpenGlyph {
			t.Fatalf("%s open mark = %q, want %q", mode, mark, glyphs.GroupOpenGlyph)
		}
		model.RestoreHostFolds([]rail.HostFold{rail.RunsFold})
		model.Reconcile(runsSnapshot())
		if mark, _ := headerParts(t, model, ctx); mark != glyphs.GroupClosedGlyph {
			t.Fatalf("%s folded mark = %q, want %q", mode, mark, glyphs.GroupClosedGlyph)
		}
	}
}

func headerClick(t *testing.T, model *rail.Model, frame rail.Frame, x int, at time.Time) rail.Decision {
	t.Helper()
	for y, line := range frame.Lines {
		if line.HitOK && line.Hit.Kind == rail.RunsRow {
			msg := tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
			return model.Mouse(msg, frame, rail.MouseContext{FullWidth: true, Now: at})
		}
	}
	t.Fatal("no Runs header")
	return rail.Decision{}
}

func TestClickingTheRunsFoldMarkTogglesTheFold(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(runsSnapshot())
	frame := model.Render(renderContext(48, 8), rail.Frame{})
	x, ok := frame.Folds["runs"]
	if !ok || len(frame.Folds) != 2 {
		t.Fatalf("fold marks = %v; want the Runs and Bots headers'", frame.Folds)
	}
	for _, line := range frame.Lines {
		if line.HitOK && line.Hit.Kind == rail.RunsRow {
			if got := ansi.Strip(ansi.Cut(line.Text, x-1, x)); got != presentation.GlyphsFor(renderContext(48, 8).GlyphMode).GroupOpenGlyph {
				t.Fatalf("fold mark column %d holds %q", x, got)
			}
		}
	}
	now := time.Now()
	decision := headerClick(t, &model, frame, x, now)
	if !runsFolded(t, model) || len(decision.Mutations) != 1 || decision.Selection.Kind != rail.RunsRow {
		t.Fatalf("a click on the mark did not fold: %+v", decision)
	}
	frame = model.Render(renderContext(48, 8), rail.Frame{})
	headerClick(t, &model, frame, x, now.Add(time.Millisecond))
	if runsFolded(t, model) {
		t.Fatal("a second click on the mark did not unfold")
	}
	// Off the mark, a single click selects and a double click folds, the
	// way it does a group.
	frame = model.Render(renderContext(48, 8), rail.Frame{})
	headerClick(t, &model, frame, x+4, now.Add(time.Second))
	if runsFolded(t, model) {
		t.Fatal("a single click off the mark folded")
	}
	headerClick(t, &model, frame, x+4, now.Add(time.Second+time.Millisecond))
	if !runsFolded(t, model) {
		t.Fatal("a double click on the header did not fold")
	}
}

func TestNoFoldMarkHitWithTheMouseOff(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(runsSnapshot())
	ctx := renderContext(48, 8)
	ctx.MouseDisabled = true
	if frame := model.Render(ctx, rail.Frame{}); len(frame.Folds) != 0 {
		t.Fatalf("folds = %v with the mouse off", frame.Folds)
	}
}
