package rail

import (
	"slices"
	"testing"

	"github.com/YoanWai/agent-manager/internal/keybind"
	tea "github.com/charmbracelet/bubbletea"
)

func menuLabels(model Model, selection Selection) []string {
	var got []string
	for _, item := range model.rowMenuItems(model.rows[model.rowIndex(selection.key())]) {
		if item.label != "" {
			got = append(got, item.label)
		}
	}
	return got
}

func TestAutomationMenusOfferEveryAutomationAction(t *testing.T) {
	model := New(nil)
	model.Reconcile(Snapshot{
		Sessions: []Session{{ID: "one", Name: "one"}},
		Automations: []Automation{
			{ID: "on", Name: "on", Mode: "work", Enabled: true},
			{ID: "off", Name: "off", Mode: "work"},
		},
	})
	if got := menuLabels(model, Selection{Kind: AutomationRow, Automation: "on"}); !slices.Equal(got, []string{"Show firings", "Edit", "Disable", "Run now", "New Automation", "Delete"}) {
		t.Errorf("enabled Automation menu = %v", got)
	}
	if got := menuLabels(model, Selection{Kind: AutomationRow, Automation: "off"}); !slices.Contains(got, "Enable") {
		t.Errorf("disabled Automation menu = %v", got)
	}
	if got := menuLabels(model, Selection{Kind: AutomationsRow}); !slices.Contains(got, "New Automation") {
		t.Errorf("Automations header menu = %v", got)
	}
	if got := menuLabels(model, Selection{Kind: GroupRow}); !slices.Contains(got, "New Automation") {
		t.Errorf("root menu = %v, want New Automation", got)
	}
	on := Selection{Kind: AutomationRow, Automation: "on"}
	model.openMenu(on, 0, 0, false)
	keys := KeyContext{ListKeys: keybind.DefaultList()}
	if decision := model.menuKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")}, keys); decision.Intent != (Intent{Kind: RunNow, Target: on}) {
		t.Errorf("! in the Automation menu = %+v", decision.Intent)
	}
}

func TestAutomationKeysAimAtAnAutomationRowOnly(t *testing.T) {
	model := New(nil)
	model.Reconcile(Snapshot{
		Sessions:    []Session{{ID: "one", Name: "one"}},
		Automations: []Automation{{ID: "on", Name: "on", Mode: "work", Enabled: true}},
	})
	keys := KeyContext{ListKeys: keybind.DefaultList()}
	press := func(key string) Intent {
		return model.Key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}, Frame{}, keys).Intent
	}
	model.Focus(Selection{Kind: SessionRow, SessionID: "one"})
	if got := press("S"); got.Kind != NewAutomation {
		t.Errorf("S on a session = %+v; want New Automation", got)
	}
	for _, key := range []string{"E", "!"} {
		if got := press(key); got.Kind != NoAction {
			t.Errorf("%s on a session = %+v; want nothing", key, got)
		}
	}
	model.Focus(Selection{Kind: AutomationRow, Automation: "on"})
	if got := press("E"); got.Kind != ToggleEnabled || got.Target.Automation != "on" {
		t.Errorf("E on an Automation = %+v", got)
	}
	if got := press("!"); got.Kind != RunNow || got.Target.Automation != "on" {
		t.Errorf("! on an Automation = %+v", got)
	}
}

func TestDebugSessionAimsAtAFiringRowAndIsInItsMenu(t *testing.T) {
	model := New(nil)
	model.Reconcile(Snapshot{
		Sessions:    []Session{{ID: "one", Name: "one"}},
		Automations: []Automation{{ID: "on", Name: "on", Mode: "work", Enabled: true, Firings: []Firing{{ID: "f1", When: "Nov 23 07:00", Status: "done", Outcome: "done"}}}},
	})
	keys := KeyContext{ListKeys: keybind.DefaultList()}
	press := func(msg tea.KeyMsg) Intent { return model.Key(msg, Frame{}, keys).Intent }
	model.Focus(Selection{Kind: AutomationRow, Automation: "on"})
	if got := press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")}); got.Kind != NoAction {
		t.Errorf("D on an Automation = %+v; want nothing", got)
	}
	model.Key(tea.KeyMsg{Type: tea.KeyEnter}, Frame{}, keys)
	firing := Selection{Kind: FiringRow, Automation: "on", Firing: "f1"}
	model.Focus(firing)
	if got := press(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")}); got != (Intent{Kind: DebugSession, Target: firing}) {
		t.Errorf("D on a firing = %+v", got)
	}
	if got := menuLabels(model, firing); !slices.Equal(got, []string{"Open log", "Debug session"}) {
		t.Errorf("firing menu = %v", got)
	}
}
