package rail_test

import (
	"strings"
	"testing"

	"github.com/YoanWai/agent-manager/internal/keybind"
	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	"github.com/YoanWai/agent-manager/internal/ui/rail"
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
	want := []rail.RowKind{rail.GroupRow, rail.SessionRow, rail.RunsRow, rail.RunRow, rail.RunRow}
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
