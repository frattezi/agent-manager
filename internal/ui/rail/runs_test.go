package rail_test

import (
	"strings"
	"testing"

	"github.com/YoanWai/agent-manager/internal/keybind"
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
