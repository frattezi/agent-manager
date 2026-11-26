package rail_test

import (
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
		Runs:     []rail.Run{{ID: "r1", Ticket: "jira:ABC-1", Status: "done"}},
		Automations: []rail.Automation{
			{ID: "nightly", Name: "Nightly", Mode: "work", Enabled: true, Last: "failed", LastStatus: "failed", Next: "07:00", Firings: []rail.Firing{
				{ID: "f2", When: "Tue 07:00", Status: "failed", Duration: "3m12s"},
				{ID: "f1", When: "Mon 07:00", Status: "done", Duration: "42s"},
			}},
			{ID: "sweep", Name: "Sweep", Mode: "personal", Last: rail.NeverFired},
		},
	}
}

func rowKeys(model rail.Model) []string {
	var got []string
	for _, row := range model.Rows() {
		got = append(got, row.Name)
	}
	return got
}

func TestAutomationsSectionFollowsTheRuns(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(automationsSnapshot())
	var kinds []rail.RowKind
	for _, row := range model.Rows() {
		kinds = append(kinds, row.Kind)
	}
	want := []rail.RowKind{rail.GroupRow, rail.SessionRow, rail.RunsRow, rail.RunRow, rail.AutomationsRow, rail.AutomationRow, rail.FiringRow, rail.FiringRow, rail.AutomationRow}
	if len(kinds) != len(want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("kinds = %v, want %v", kinds, want)
		}
	}
	if got := strings.Join(rowKeys(model)[4:], ","); got != "automations,Nightly,Tue 07:00,Mon 07:00,Sweep" {
		t.Fatalf("names = %s", got)
	}
}

func TestNoAutomationsNoSection(t *testing.T) {
	model := rail.New(nil)
	snapshot := automationsSnapshot()
	snapshot.Automations = nil
	model.Reconcile(snapshot)
	for _, row := range model.Rows() {
		if row.Kind == rail.AutomationsRow || row.Kind == rail.AutomationRow {
			t.Fatalf("listed %+v with no Automations", row)
		}
	}
}

func TestAutomationRowsPaintModeLastOutcomeAndNextFire(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(automationsSnapshot())
	frame := model.Render(renderContext(48, 16), rail.Frame{})
	var text string
	for _, line := range frame.Lines {
		text += strings.TrimRight(ansi.Strip(line.Text), " ") + "\n"
	}
	for _, want := range []string{"Automations (2)", "Nightly", "failed · next 07:00", "Tue 07:00", "3m12s", "Sweep", "never fired · disabled"} {
		if !strings.Contains(text, want) {
			t.Fatalf("frame lacks %q:\n%s", want, text)
		}
	}
	for _, line := range frame.Lines {
		plain := ansi.Strip(line.Text)
		if strings.Contains(plain, "Nightly") && !strings.Contains(plain, " W") || strings.Contains(plain, "Sweep") && !strings.Contains(plain, " P") {
			t.Fatalf("row lacks its Mode badge: %q", plain)
		}
	}
}

func TestEnterOnAFiringAsksForItsLog(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(automationsSnapshot())
	model.Focus(rail.Selection{Kind: rail.FiringRow, Automation: "nightly", Firing: "f1"})
	decision := model.Key(keyFor("enter"), rail.Frame{}, rail.KeyContext{ListKeys: keybind.DefaultList()})
	target := decision.Intent.Target
	if decision.Intent.Kind != rail.Attach || target.Kind != rail.FiringRow || target.Firing != "f1" || target.Automation != "nightly" {
		t.Fatalf("intent = %+v", decision.Intent)
	}
}

func TestDoubleClickOnAFiringAsksForItsLog(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(automationsSnapshot())
	frame := model.Render(renderContext(48, 16), rail.Frame{})
	for y, line := range frame.Lines {
		if !line.HitOK || line.Hit.Kind != rail.FiringRow || line.Hit.Firing != "f2" {
			continue
		}
		now := time.Now()
		msg := tea.MouseMsg{X: 8, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
		first := model.Mouse(msg, frame, rail.MouseContext{FullWidth: true, FullLayout: true, Now: now})
		if first.Intent.Kind == rail.Focus {
			t.Fatal("a single click opened the log")
		}
		second := model.Mouse(msg, frame, rail.MouseContext{FullWidth: true, FullLayout: true, Now: now.Add(time.Millisecond)})
		if second.Intent.Kind != rail.Focus || second.Intent.Target.Firing != "f2" {
			t.Fatalf("double click = %+v", second.Intent)
		}
		return
	}
	t.Fatal("no firing row in the frame")
}

func TestClickingTheAutomationsFoldMarkFoldsTheSection(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(automationsSnapshot())
	frame := model.Render(renderContext(48, 16), rail.Frame{})
	for y, line := range frame.Lines {
		if !line.HitOK || line.Hit.Kind != rail.AutomationsRow {
			continue
		}
		x, ok := frame.Folds["automations"]
		if !ok {
			t.Fatalf("no fold mark for the header in %v", frame.Folds)
		}
		decision := model.Mouse(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}, frame, rail.MouseContext{FullWidth: true, Now: time.Now()})
		if len(decision.Mutations) != 1 || !model.SelectionCollapsed(rail.Selection{Kind: rail.AutomationsRow}) {
			t.Fatalf("click on the fold mark: %+v", decision)
		}
		for _, row := range model.Rows() {
			if row.Kind == rail.AutomationRow || row.Kind == rail.FiringRow {
				t.Fatalf("a folded section listed %+v", row)
			}
		}
		restored := rail.New(nil)
		restored.RestoreHostFolds(model.HostCollapsed())
		if !restored.SelectionCollapsed(rail.Selection{Kind: rail.AutomationsRow}) {
			t.Fatal("the fold did not survive a restore")
		}
		return
	}
	t.Fatal("no Automations header")
}

func TestAnAutomationCannotBeReordered(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(automationsSnapshot())
	model.Focus(rail.Selection{Kind: rail.AutomationRow, Automation: "sweep"})
	decision := model.Key(keyFor("shift+up"), rail.Frame{}, rail.KeyContext{ListKeys: keybind.DefaultList()})
	if len(decision.Mutations) != 0 || decision.Error == "" {
		t.Fatalf("reordering an Automation: %+v", decision)
	}
}

func TestSearchNarrowsAutomationsToTheirNames(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(automationsSnapshot())
	model.SetSearch("sweep", false)
	var listed []string
	for _, row := range model.Rows() {
		if row.Kind == rail.AutomationRow || row.Kind == rail.FiringRow {
			listed = append(listed, row.Name)
		}
	}
	if len(listed) != 1 || listed[0] != "Sweep" {
		t.Fatalf("listed after search = %v", listed)
	}
}
