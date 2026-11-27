package rail_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/keybind"
	"github.com/YoanWai/agent-manager/internal/ui/rail"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func automationsSnapshot() rail.Snapshot {
	return rail.Snapshot{
		Sessions: []rail.Session{{ID: "one", Name: "one"}},
		Automations: []rail.Automation{
			{ID: "report", Name: "weekly report", Mode: "work", Enabled: true, Last: "done", LastStatus: "done", Next: "Mon 07:00", Firings: []rail.Firing{
				{ID: "f2", When: "2026-11-23 07:00", Status: "done", Outcome: "done"},
				{ID: "f1", When: "2026-11-16 07:00", Status: "failed", Outcome: "failed"},
			}},
			{ID: "sweep", Name: "monday sweep", Mode: "personal", Enabled: false, Last: "failed", LastStatus: "failed"},
			{ID: "fresh", Name: "fresh chore", Mode: "work", Enabled: true, Next: "07:00"},
		},
	}
}

func rowKinds(model rail.Model) []rail.RowKind {
	var kinds []rail.RowKind
	for _, row := range model.Rows() {
		kinds = append(kinds, row.Kind)
	}
	return kinds
}

func listKeys() rail.KeyContext { return rail.KeyContext{ListKeys: keybind.DefaultList()} }

func TestAutomationsListFoldedAfterTheSessions(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(automationsSnapshot())
	want := []rail.RowKind{rail.GroupRow, rail.SessionRow, rail.AutomationsRow, rail.AutomationRow, rail.AutomationRow, rail.AutomationRow, rail.BotsRow}
	if got := rowKinds(model); !slices.Equal(got, want) {
		t.Fatalf("kinds = %v, want %v", got, want)
	}
}

func TestNoAutomationsNoSection(t *testing.T) {
	model := rail.New(nil)
	snapshot := automationsSnapshot()
	snapshot.Automations = nil
	model.Reconcile(snapshot)
	if got := rowKinds(model); slices.Contains(got, rail.AutomationsRow) {
		t.Fatalf("kinds = %v with no Automations", got)
	}
}

func TestEnterUnfoldsAnAutomationAndAsksForAFiringsLog(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(automationsSnapshot())
	model.Focus(rail.Selection{Kind: rail.AutomationRow, Automation: "report"})
	if decision := model.Key(keyFor("enter"), rail.Frame{}, listKeys()); decision.Intent.Kind != 0 || len(decision.Mutations) != 1 {
		t.Fatalf("enter on an Automation: %+v", decision)
	}
	var firings []string
	for _, row := range model.Rows() {
		if row.Kind == rail.FiringRow {
			firings = append(firings, row.Firing)
		}
	}
	if !slices.Equal(firings, []string{"f2", "f1"}) {
		t.Fatalf("firings = %v", firings)
	}
	model.Focus(rail.Selection{Kind: rail.FiringRow, Automation: "report", Firing: "f1"})
	decision := model.Key(keyFor("enter"), rail.Frame{}, listKeys())
	target := decision.Intent.Target
	if decision.Intent.Kind != rail.Attach || target.Kind != rail.FiringRow || target.Automation != "report" || target.Firing != "f1" {
		t.Fatalf("enter on a firing: %+v", decision.Intent)
	}
	model.Focus(rail.Selection{Kind: rail.AutomationRow, Automation: "report"})
	model.Key(keyFor("enter"), rail.Frame{}, listKeys())
	if slices.Contains(rowKinds(model), rail.FiringRow) {
		t.Fatal("a second enter left the firings listed")
	}
}

func TestAnAutomationCannotBeReordered(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(automationsSnapshot())
	model.Focus(rail.Selection{Kind: rail.AutomationRow, Automation: "sweep"})
	if decision := model.Key(keyFor("shift+up"), rail.Frame{}, listKeys()); len(decision.Mutations) != 0 || decision.Error == "" {
		t.Fatalf("reordering an Automation: %+v", decision)
	}
}

func TestSearchAndAttentionNarrowTheAutomations(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(automationsSnapshot())
	names := func() []string {
		var out []string
		for _, row := range model.Rows() {
			if row.Kind == rail.AutomationRow {
				out = append(out, row.Name)
			}
		}
		return out
	}
	model.SetSearch("sweep", false)
	if got := names(); !slices.Equal(got, []string{"monday sweep"}) {
		t.Fatalf("search = %v", got)
	}
	model.SetSearch("", false)
	model.SetFilteringAttention(true)
	if got := names(); !slices.Equal(got, []string{"monday sweep"}) {
		t.Fatalf("attention = %v", got)
	}
}

func TestAutomationRowsPaintModeLastOutcomeAndNextFire(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(automationsSnapshot())
	frame := model.Render(renderContext(48, 12), rail.Frame{})
	var text string
	for _, line := range frame.Lines {
		text += ansi.Strip(line.Text) + "\n"
	}
	for _, want := range []string{"Automations (3)", "weekly report", "done · next Mon 07:00", "monday sweep", "failed · disabled", "fresh chore", "never fired · next 07:00"} {
		if !strings.Contains(text, want) {
			t.Fatalf("frame lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "disabled · next") {
		t.Fatalf("a disabled Automation shows a next fire time:\n%s", text)
	}
}

func TestClickingAnAutomationsFoldMarkListsItsFirings(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(automationsSnapshot())
	frame := model.Render(renderContext(48, 12), rail.Frame{})
	if len(frame.Folds) != 5 {
		t.Fatalf("fold marks = %v; want the header's, one per Automation and the Bots header's", frame.Folds)
	}
	x, ok := frame.Folds["a:report"]
	if !ok {
		t.Fatalf("no fold mark for report in %v", frame.Folds)
	}
	for y, line := range frame.Lines {
		if line.HitOK && line.Hit == (rail.Selection{Kind: rail.AutomationRow, Automation: "report"}) {
			msg := tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
			model.Mouse(msg, frame, rail.MouseContext{FullWidth: true, Now: time.Now()})
			break
		}
	}
	if !slices.Contains(rowKinds(model), rail.FiringRow) {
		t.Fatalf("a click on the mark left the firings folded: %v", rowKinds(model))
	}
}
