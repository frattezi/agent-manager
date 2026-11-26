package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/ui/automations"
	uiautomationsform "github.com/YoanWai/agent-manager/internal/ui/automations/form"
	uidialogs "github.com/YoanWai/agent-manager/internal/ui/dialogs"
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func fractoryRepoDir(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".fractory.json"), []byte(`{"mode":"work"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

func automationDialog(t *testing.T, m *Model) *uiautomationsform.Surface {
	t.Helper()
	s, ok := surfaceOf[*uiautomationsform.Surface](m)
	if !ok {
		t.Fatal("the Automations dialog is not open")
	}
	return s
}

// fillAutomation tabs to each field and types its value over what it holds.
func fillAutomation(t *testing.T, m *Model, values map[int]string) {
	t.Helper()
	for field := range uiautomationsform.RetentionField + 1 {
		value, ok := values[field]
		if !ok {
			continue
		}
		for automationDialog(t, m).Dialog().Focus() != field {
			m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyTab})
		}
		m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyCtrlU})
		m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyCtrlK})
		if value != "" {
			m.applyTestMsg(t, runeKey(value))
		}
	}
}

func validAutomation(repo string) map[int]string {
	return map[int]string{
		uiautomationsform.NameField:      "weekly report",
		uiautomationsform.DirField:       repo,
		uiautomationsform.CronField:      "0 7 1 1 *",
		uiautomationsform.TimezoneField:  "Europe/Lisbon",
		uiautomationsform.EntryField:     "prompts/weekly.md",
		uiautomationsform.FlagsField:     "--model opus",
		uiautomationsform.RetentionField: "4",
	}
}

func saveAutomation(t *testing.T, m *Model) {
	t.Helper()
	m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyEnter})
	m.drainEffects(t)
}

func storedConfig(t *testing.T, m *Model, id string) automations.Config {
	t.Helper()
	a, ok, err := m.services.store.Automation(id)
	if err != nil || !ok {
		t.Fatalf("Automation %q: found %v, %v", id, ok, err)
	}
	var config automations.Config
	if err := json.Unmarshal(a.Config, &config); err != nil {
		t.Fatal(err)
	}
	return config
}

func onlyAutomation(t *testing.T, m *Model) string {
	t.Helper()
	all, err := m.services.store.Automations()
	if err != nil || len(all) != 1 {
		t.Fatalf("stored %d Automations (%v); want one", len(all), err)
	}
	return all[0].ID
}

func TestAutomationDialogStoresAnAutomationAndEditsItInPlace(t *testing.T) {
	fractoryHome(t)
	m := buildModel(t)
	repo := fractoryRepoDir(t)
	m.applyTestMsg(t, runeKey("S"))
	if view := ansi.Strip(preparedView(m)); !strings.Contains(view, "New Automation") {
		t.Fatalf("S did not open the dialog:\n%s", view)
	}
	fillAutomation(t, m, validAutomation(repo))
	saveAutomation(t, m)
	if _, open := surfaceOf[*uiautomationsform.Surface](m); open {
		t.Fatalf("a stored Automation left the dialog open; status %q", m.errBar.text)
	}
	id := onlyAutomation(t, m)
	config := storedConfig(t, m, id)
	want := automations.Config{Version: 1, ID: id, Name: "weekly report", Kind: "agent", RepoPath: repo, Cron: "0 7 1 1 *", Timezone: "Europe/Lisbon",
		Mode: "work", Tool: m.services.cfg.AgentToolNames()[0], Flags: []string{"--model", "opus"}, Entry: "prompts/weekly.md", Retention: 4}
	if got, _ := json.Marshal(config); string(got) != string(must(json.Marshal(want))) {
		t.Fatalf("stored %s\nwant   %s", got, must(json.Marshal(want)))
	}
	if selection, _ := m.rail.Selected(); selection.Kind != uirail.AutomationRow || selection.Automation != id {
		t.Fatalf("the cursor is on %+v; want the new Automation", selection)
	}
	if view := ansi.Strip(preparedView(m)); !strings.Contains(view, "weekly report") {
		t.Fatalf("the rail does not list the new Automation:\n%s", view)
	}

	fired := time.Date(2026, 1, 1, 7, 0, 0, 0, time.UTC)
	if err := m.services.store.SetAutomationLastFired(id, fired); err != nil {
		t.Fatal(err)
	}
	if err := m.services.store.SetAutomationEnabled(id, false, time.Now()); err != nil {
		t.Fatal(err)
	}
	checkAutomations(t, m)
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: id})
	m.applyTestMsg(t, runeKey("r"))
	view := ansi.Strip(preparedView(m))
	for _, shown := range []string{"Edit Automation", "weekly report", "0 7 1 1 *", "Europe/Lisbon", "prompts/weekly.md", "--model opus", "4"} {
		if !strings.Contains(view, shown) {
			t.Fatalf("the edit dialog does not show %q:\n%s", shown, view)
		}
	}
	fillAutomation(t, m, map[int]string{uiautomationsform.NameField: "monday sweep", uiautomationsform.CronField: "30 6 * * 1"})
	automationDialog(t, m).Dialog().Click(automationLineOf(t, m, uiautomationsform.ModeField))
	automationDialog(t, m).Dialog().Click(automationLineOf(t, m, uiautomationsform.ModeField))
	saveAutomation(t, m)
	a, _, _ := m.services.store.Automation(id)
	config = storedConfig(t, m, id)
	if onlyAutomation(t, m) != id || config.ID != id || config.Name != "monday sweep" || config.Cron != "30 6 * * 1" || config.Mode != "personal" {
		t.Fatalf("edited to %+v", config)
	}
	if a.Name != "monday sweep" || a.Enabled || !a.LastFiredFor.Equal(fired) {
		t.Fatalf("the edit moved the state: %+v", a)
	}
}

func must(data []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return data
}

// automationLineOf is the body line the dialog paints field on.
func automationLineOf(t *testing.T, m *Model, field int) int {
	t.Helper()
	preparedView(m)
	dialog := automationDialog(t, m).Dialog()
	focus := dialog.Focus()
	for line := range 40 {
		probe := *dialog
		probe.Click(line)
		if probe.Focus() == field && (focus != field || line >= 0) {
			return line
		}
	}
	t.Fatalf("no line paints field %d", field)
	return -1
}

func TestAutomationDialogRefusesWhatCouldNeverRunAgainstItsField(t *testing.T) {
	for name, tc := range map[string]struct {
		field  int
		value  func(t *testing.T) string
		reason string
	}{
		"malformed cron":            {uiautomationsform.CronField, func(*testing.T) string { return "every monday" }, "want 5"},
		"missing entry point":       {uiautomationsform.EntryField, func(*testing.T) string { return "" }, "entry is empty"},
		"not a Fractory repository": {uiautomationsform.DirField, func(t *testing.T) string { return t.TempDir() }, "not a Fractory repository"},
	} {
		t.Run(name, func(t *testing.T) {
			fractoryHome(t)
			m := buildModel(t)
			values := validAutomation(fractoryRepoDir(t))
			values[tc.field] = tc.value(t)
			m.applyTestMsg(t, runeKey("S"))
			fillAutomation(t, m, values)
			saveAutomation(t, m)
			dialog := automationDialog(t, m).Dialog()
			if refusal := dialog.Refusal(); refusal.Field != tc.field || !strings.Contains(refusal.Reason, tc.reason) {
				t.Fatalf("refusal = %+v; want %q on field %d", refusal, tc.reason, tc.field)
			}
			if dialog.Focus() != tc.field {
				t.Fatalf("focus = %d; want the field at fault", dialog.Focus())
			}
			label := map[int]string{uiautomationsform.CronField: "cron", uiautomationsform.EntryField: "entry", uiautomationsform.DirField: "directory"}[tc.field]
			lines := strings.Split(ansi.Strip(preparedView(m)), "\n")
			at := slices.IndexFunc(lines, func(line string) bool { return strings.Contains(line, "❯ "+label+" ") })
			if at < 0 || at+1 >= len(lines) || !strings.HasPrefix(strings.TrimLeft(strings.TrimPrefix(strings.TrimSpace(lines[at+1]), "│"), " "), label+" ") {
				t.Fatalf("the reason is not under its field:\n%s", strings.Join(lines, "\n"))
			}
			if all, _ := m.services.store.Automations(); len(all) != 0 {
				t.Fatalf("a refused Automation was stored: %+v", all)
			}
			fillAutomation(t, m, map[int]string{tc.field: validAutomation(fractoryRepoDir(t))[tc.field]})
			saveAutomation(t, m)
			onlyAutomation(t, m)
		})
	}
}

func TestAutomationRowEnablesDisablesAndDeletesBehindAConfirm(t *testing.T) {
	m, _, _ := automationFixtures(t)
	checkAutomations(t, m)
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "report"})
	m.applyTestMsg(t, runeKey("E"))
	m.drainEffects(t)
	if a, _, _ := m.services.store.Automation("report"); a.Enabled {
		t.Fatal("E left the Automation enabled")
	}
	if view := ansi.Strip(preparedView(m)); !strings.Contains(view, "running · disabled") {
		t.Fatalf("the row does not say it is disabled:\n%s", view)
	}
	m.applyTestMsg(t, runeKey("E"))
	m.drainEffects(t)
	if a, _, _ := m.services.store.Automation("report"); !a.Enabled || len(a.Config) == 0 {
		t.Fatalf("E did not enable it again: %+v", a)
	}

	m.applyTestMsg(t, runeKey("d"))
	confirm, ok := surfaceOf[*uidialogs.ConfirmSurface](m)
	if !ok || confirm.Prompt().Action != uidialogs.ConfirmDeleteAutomation {
		t.Fatal("d did not ask before deleting")
	}
	m.applyTestMsg(t, runeKey("n"))
	m.drainEffects(t)
	if _, found, _ := m.services.store.Automation("report"); !found {
		t.Fatal("a cancelled delete removed the Automation")
	}
	m.applyTestMsg(t, runeKey("d"))
	m.applyTestMsg(t, runeKey("y"))
	m.drainEffects(t)
	if _, found, _ := m.services.store.Automation("report"); found {
		t.Fatal("a confirmed delete kept the Automation")
	}
	if slices.ContainsFunc(automationRowsOf(m), func(row uirail.Row) bool { return row.Automation == "report" }) {
		t.Fatal("the deleted Automation is still listed")
	}
}

func TestAutomationRunNowRequestsTheCurrentInstantAndSkipsWhileBusy(t *testing.T) {
	home := fractoryHome(t)
	m := buildModel(t)
	addViewAutomation(t, m, "fresh", "fresh chore", "work", true, time.Now().Add(-time.Hour))
	checkAutomations(t, m)
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "fresh"})
	before := time.Now().Truncate(time.Second)
	m.applyTestMsg(t, runeKey("!"))
	m.drainEffects(t)
	entries, _ := os.ReadDir(filepath.Join(home, "automations"))
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "fresh@") {
		t.Fatalf("run now wrote %v", entries)
	}
	data, _ := os.ReadFile(filepath.Join(home, "automations", entries[0].Name()))
	var request automations.Request
	if err := json.Unmarshal(data, &request); err != nil || request.FiredFor.Before(before) || request.FiredFor.After(time.Now()) {
		t.Fatalf("request %s: %v", data, err)
	}
	if a, _, _ := m.services.store.Automation("fresh"); !a.LastFiredFor.IsZero() {
		t.Fatalf("run now advanced the watermark to %v", a.LastFiredFor)
	}
	if !strings.Contains(m.errBar.text, "fresh chore") {
		t.Fatalf("status = %q", m.errBar.text)
	}

	m.applyTestMsg(t, runeKey("!"))
	m.drainEffects(t)
	if entries, _ := os.ReadDir(filepath.Join(home, "automations")); len(entries) != 1 {
		t.Fatalf("a run now while the first is queued wrote %d requests", len(entries))
	}
	if !strings.Contains(m.errBar.text, "skipped") {
		t.Fatalf("status = %q; want the skip reported", m.errBar.text)
	}
}

func TestEveryAutomationActionIsInTheFooter(t *testing.T) {
	m, _, _ := automationFixtures(t)
	checkAutomations(t, m)
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "report"})
	footer := ansi.Strip(m.viewFooter())
	for _, want := range []string{"r edit", "E disable", "! run now", "d delete"} {
		if !strings.Contains(footer, want) {
			t.Fatalf("footer lacks %q:\n%s", want, footer)
		}
	}
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "sweep"})
	if footer := ansi.Strip(m.viewFooter()); !strings.Contains(footer, "E enable") {
		t.Fatalf("a disabled Automation's footer:\n%s", footer)
	}
}
