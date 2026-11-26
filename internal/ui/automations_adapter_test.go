package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/keybind"
	"github.com/YoanWai/agent-manager/internal/store"
	"github.com/YoanWai/agent-manager/internal/ui/automations"
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	"github.com/YoanWai/agent-manager/internal/ui/runs"
	uilaunch "github.com/YoanWai/agent-manager/internal/ui/spawn/launch"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func automationConfig(t *testing.T, id, name, mode string) []byte {
	t.Helper()
	data, err := automations.Config{
		Version: 1, ID: id, Name: name, Kind: automations.KindScript, RepoPath: "/repo/" + id, Cron: "0 7 * * *",
		Timezone: "UTC", Mode: mode, Entry: "scripts/" + id + ".sh", Retention: 5,
	}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// storeAutomations adds a nightly, enabled Work Automation and a disabled
// Personal one, in that order.
func storeAutomations(t *testing.T, m *Model) []store.Automation {
	t.Helper()
	now := time.Now()
	for _, a := range []store.Automation{
		{ID: "nightly", Name: "Nightly report", Config: automationConfig(t, "nightly", "Nightly report", automations.ModeWork), Enabled: true},
		{ID: "sweep", Name: "Monday sweep", Config: automationConfig(t, "sweep", "Monday sweep", automations.ModePersonal)},
	} {
		if err := m.services.store.AddAutomation(a, now); err != nil {
			t.Fatal(err)
		}
	}
	stored, err := m.services.store.Automations()
	if err != nil {
		t.Fatal(err)
	}
	return stored
}

func testFirings() []automations.Record {
	day := time.Date(2026, 11, 25, 7, 0, 0, 0, time.UTC)
	return []automations.Record{
		{Version: 1, ID: "run-3", AutomationID: "nightly", FiredFor: day, Status: automations.StatusRunning, StartedAt: time.Now().Add(-90 * time.Second), LogPath: "/tmp/automation logs/run-3.log"},
		{Version: 1, ID: "run-2", AutomationID: "nightly", FiredFor: day.Add(-24 * time.Hour), Status: "failed", StartedAt: day.Add(-24 * time.Hour),
			UpdatedAt: day.Add(-24*time.Hour + 3*time.Minute + 12*time.Second), LogPath: "/tmp/run-2.log", Result: &automations.Result{Outcome: "failed", Summary: "the report query timed out"}},
		{Version: 1, ID: "run-1", AutomationID: "nightly", FiredFor: day.Add(-48 * time.Hour), Status: "done", StartedAt: day.Add(-48 * time.Hour),
			UpdatedAt: day.Add(-48*time.Hour + 42*time.Second), LogPath: "/tmp/run-1.log"},
	}
}

func loadAutomations(t *testing.T, m *Model, records []automations.Record) {
	t.Helper()
	stored, err := m.services.store.Automations()
	if err != nil {
		t.Fatal(err)
	}
	m.applyTestMsg(t, automationsLoadedMsg{stored: stored, records: records})
}

func automationRows(m *Model) []uirail.Row {
	var rows []uirail.Row
	for _, row := range m.rail.Rows() {
		if isAutomationKind(row.Kind) {
			rows = append(rows, row)
		}
	}
	return rows
}

func detailText(t *testing.T, m *Model) string {
	t.Helper()
	lines, ok := m.automationContentLines(100)
	if !ok {
		t.Fatal("no detail for the selected Automation row")
	}
	var text []string
	for _, line := range lines {
		text = append(text, ansi.Strip(line.Text))
	}
	return strings.Join(text, "\n")
}

func TestAutomationsSectionIsHiddenWithoutAutomations(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadAutomations(t, m, testFirings())
	if rows := automationRows(m); len(rows) != 0 {
		t.Fatalf("an empty store listed %+v", rows)
	}
}

func TestAutomationsSectionListsEachAutomationWithItsFirings(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	storeAutomations(t, m)
	loadAutomations(t, m, testFirings())
	var got []string
	for _, row := range automationRows(m) {
		got = append(got, row.Selection.Automation+"/"+row.Selection.Firing)
	}
	want := []string{"/", "nightly/", "nightly/run-3", "nightly/run-2", "nightly/run-1", "sweep/"}
	if !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	if rows := m.rail.Rows(); rows[len(rows)-1].Kind != uirail.AutomationRow {
		t.Fatalf("the section is not last: %+v", rows[len(rows)-1])
	}
}

func TestAutomationRowsShowNameModeLastOutcomeAndNextFire(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	storeAutomations(t, m)
	loadAutomations(t, m, testFirings()[1:])
	m.layout.width, m.layout.height = 140, 30
	view := ansi.Strip(preparedView(m))
	next, err := automations.NextFire(automations.Config{Cron: "0 7 * * *", Timezone: "UTC"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Automations (2)", "Nightly report", "failed · next " + clockNear(next, time.Now()), "Monday sweep", "never fired · disabled"} {
		if !strings.Contains(view, want) {
			t.Fatalf("frame lacks %q:\n%s", want, view)
		}
	}
}

func TestSelectingAnAutomationListsItsFiringsNewestFirst(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	storeAutomations(t, m)
	firings := testFirings()
	loadAutomations(t, m, firings)
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "nightly"})
	detail := detailText(t, m)
	for _, want := range []string{"Automation Nightly report", "Mode work", "Directory /repo/nightly", "Entry scripts/nightly.sh", "Schedule 0 7 * * * UTC", "Firings, newest first"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("detail lacks %q:\n%s", want, detail)
		}
	}
	now := time.Now()
	wants := []string{
		clockNear(firings[0].FiredFor, now) + "  running  1m3",
		clockNear(firings[1].FiredFor, now) + "  failed  3m12s",
		clockNear(firings[2].FiredFor, now) + "  done  42s",
	}
	last := -1
	for _, want := range wants {
		at := strings.Index(detail, want)
		if at <= last {
			t.Fatalf("firing %q missing or out of order:\n%s", want, detail)
		}
		last = at
	}
}

func TestAnAutomationThatNeverFiredSaysSo(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	storeAutomations(t, m)
	loadAutomations(t, m, nil)
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "sweep"})
	detail := detailText(t, m)
	if !strings.Contains(detail, "Never fired") || !strings.Contains(detail, "Next disabled") {
		t.Fatalf("detail = %s", detail)
	}
	cmd, taken := m.automationIntent(uirail.Intent{Kind: uirail.Attach, Target: m.railSelection()})
	if !taken || cmd != nil || !strings.Contains(m.errBar.text, "not fired yet") || m.effects.Main.Active != nil {
		t.Fatalf("open on a never-fired Automation: taken %v, status %q", taken, m.errBar.text)
	}
}

func TestSelectingAFiringShowsItsFactsUnderTheHeldLine(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	storeAutomations(t, m)
	m.applyTestMsg(t, runsLoadedMsg{dir: "/registry", host: &runs.HostState{Held: true, Reason: "Docker is not running"}})
	loadAutomations(t, m, testFirings())
	m.rail.Focus(uirail.Selection{Kind: uirail.FiringRow, Automation: "nightly", Firing: "run-2"})
	detail := detailText(t, m)
	if !strings.HasPrefix(detail, "  Held: Docker is not running") {
		t.Fatalf("detail does not start with the Held line:\n%s", detail)
	}
	for _, want := range []string{"Automation Nightly report", "Status failed", "Took 3m12s", "Log /tmp/run-2.log", "the report query timed out"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("detail lacks %q:\n%s", want, detail)
		}
	}
}

func TestOpeningAFiringTailsItsLog(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	storeAutomations(t, m)
	loadAutomations(t, m, testFirings())
	m.rail.Focus(uirail.Selection{Kind: uirail.FiringRow, Automation: "nightly", Firing: "run-3"})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on a firing returned no command")
	}
	request, ok := m.effects.Main.Active.Request.(uilaunch.Request)
	if !ok || request.Kind != uilaunch.Shell || request.Plan == nil {
		t.Fatalf("queued %+v; want a shell spawn with a plan", m.effects.Main.Active)
	}
	if want := "tail -n 200 -F '/tmp/automation logs/run-3.log'"; request.Plan.Command != want {
		t.Fatalf("command = %q, want %q", request.Plan.Command, want)
	}
	if request.Dir != "/tmp/automation logs" || request.Name != "log-Nightly-report-run-" {
		t.Fatalf("dir %q, name %q", request.Dir, request.Name)
	}
}

func TestOpeningAnAutomationRowTailsItsLatestFiring(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	storeAutomations(t, m)
	loadAutomations(t, m, testFirings()[1:])
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "nightly"})
	// A double click asks for Focus.
	if _, taken := m.automationIntent(uirail.Intent{Kind: uirail.Focus, Target: m.railSelection()}); !taken {
		t.Fatal("focus on an Automation row was not taken")
	}
	request, ok := m.effects.Main.Active.Request.(uilaunch.Request)
	if !ok || request.Plan == nil || request.Plan.Command != "tail -n 200 -F '/tmp/run-2.log'" {
		t.Fatalf("queued %+v", m.effects.Main.Active)
	}
}

func TestAutomationsRefuseTheKeysThatChangeASession(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	storeAutomations(t, m)
	loadAutomations(t, m, testFirings())
	m.rail.Focus(uirail.Selection{Kind: uirail.FiringRow, Automation: "nightly", Firing: "run-1"})
	for _, kind := range []uirail.ActionKind{uirail.Kill, uirail.Delete, uirail.Archive, uirail.RenameAction, uirail.Fork, uirail.Restart} {
		m.clearErr()
		if _, taken := m.automationIntent(uirail.Intent{Kind: kind, Target: m.railSelection()}); !taken || m.errBar.text == "" {
			t.Fatalf("intent %d on a firing: taken %v, status %q", kind, taken, m.errBar.text)
		}
	}
	if _, taken := m.automationIntent(uirail.Intent{Kind: uirail.Quit, Target: m.railSelection()}); taken {
		t.Fatal("quit must stay with the manager")
	}
	if len(m.workspace.sessions) != 0 || m.effects.Main.Active != nil {
		t.Fatal("a refused key changed the workspace")
	}
}

func TestAutomationLegendNamesTheLogKey(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	storeAutomations(t, m)
	loadAutomations(t, m, testFirings())
	m.rail.Focus(uirail.Selection{Kind: uirail.FiringRow, Automation: "nightly", Firing: "run-1"})
	legend, ok := m.automationLegend()
	if !ok || legend.Title != "Firing" || !slices.Contains(legend.Pairs, [2]string{m.listGlyph(keybind.Open, keybind.Attach), "open log"}) ||
		!slices.Contains(legend.Pairs, [2]string{"double click", "open log"}) {
		t.Fatalf("legend = %+v, %v", legend, ok)
	}
	if footer := ansi.Strip(m.viewFooter()); !strings.Contains(footer, "open log") {
		t.Fatalf("footer lacks the log key:\n%s", footer)
	}
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "sweep"})
	if legend, _ := m.automationLegend(); len(legend.Pairs) != 0 {
		t.Fatalf("a never-fired Automation offers %+v", legend.Pairs)
	}
}

func TestEnterFoldsTheAutomationsSection(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	storeAutomations(t, m)
	loadAutomations(t, m, testFirings())
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationsRow})
	if legend, _ := m.automationLegend(); !slices.Contains(legend.Pairs, [2]string{m.listGlyph(keybind.Open), "fold"}) {
		t.Fatalf("header legend = %+v", legend)
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.drainEffects(t)
	m.applyCmd(t, cmd)
	m.drainEffects(t)
	if rows := automationRows(m); len(rows) != 1 || rows[0].Kind != uirail.AutomationsRow {
		t.Fatalf("a folded section listed %+v", rows)
	}
	if view := ansi.Strip(preparedView(m)); !strings.Contains(view, "▸ Automations (2)") {
		t.Fatalf("folded header:\n%s", view)
	}
}

func TestAutomationsRefreshAsRecordsChange(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	storeAutomations(t, m)
	loadAutomations(t, m, testFirings()[2:])
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "nightly"})
	if strings.Contains(detailText(t, m), "failed") {
		t.Fatal("a firing listed before its record")
	}
	_, cmd, _ := m.routeAutomationsMsg(automationsLoadedMsg{stored: m.workspace.automations.stored, records: testFirings()[1:]})
	if cmd == nil {
		t.Fatal("a load did not schedule the next poll")
	}
	if !strings.Contains(detailText(t, m), "failed  3m12s") {
		t.Fatalf("the detail did not refresh:\n%s", detailText(t, m))
	}
	if got := m.railSelection(); got.Kind != uirail.AutomationRow || got.Automation != "nightly" {
		t.Fatalf("selection = %+v after a refresh", got)
	}
	if rows := automationRows(m); len(rows) != 5 {
		t.Fatalf("rows after a new record = %+v", rows)
	}
}

func writeRecordFile(t *testing.T, dir string, record automations.Record) {
	t.Helper()
	data := `{"version":1,"id":"` + record.ID + `","automationId":"` + record.AutomationID + `","firedFor":"` + record.FiredFor.Format(time.RFC3339) +
		`","status":"` + record.Status + `","logPath":"` + record.LogPath + `","later":"ignored"}`
	if err := os.WriteFile(filepath.Join(dir, record.ID+".json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

// The load reads FRACTORY_HOME, so these stay serial.
func TestAnAbsentOrEmptyRecordsDirectoryShowsTheAutomationsWithoutAnError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FRACTORY_HOME", home)
	m := buildModel(t)
	storeAutomations(t, m)
	for _, step := range []string{"absent", "empty"} {
		if step == "empty" {
			if err := os.Mkdir(filepath.Join(home, "automation-runs"), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		m.applyTestMsg(t, m.automationsLoadCmd()())
		if rows := automationRows(m); len(rows) != 3 {
			t.Fatalf("%s: rows = %+v", step, rows)
		}
		if m.errBar.text != "" {
			t.Fatalf("%s: status %q", step, m.errBar.text)
		}
		m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "nightly"})
		if detail := detailText(t, m); !strings.Contains(detail, "Never fired") {
			t.Fatalf("%s: detail = %s", step, detail)
		}
	}
}

func TestRecordsOnDiskReachTheRailAndTheLogTab(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FRACTORY_HOME", home)
	dir := filepath.Join(home, "automation-runs")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	m := buildModel(t)
	storeAutomations(t, m)
	logPath := filepath.Join(home, "logs", "abcd1234.log")
	writeRecordFile(t, dir, automations.Record{ID: "abcd1234", AutomationID: "nightly", FiredFor: time.Date(2026, 11, 26, 7, 0, 0, 0, time.UTC), Status: "done", LogPath: logPath})
	writeRecordFile(t, dir, automations.Record{ID: "older", AutomationID: "nightly", FiredFor: time.Date(2026, 11, 25, 7, 0, 0, 0, time.UTC), Status: "failed", LogPath: "/tmp/older.log"})
	m.applyTestMsg(t, m.automationsLoadCmd()())
	if !m.rail.Focus(uirail.Selection{Kind: uirail.FiringRow, Automation: "nightly", Firing: "abcd1234"}) {
		t.Fatalf("no firing row in %+v", automationRows(m))
	}
	if rows := automationRows(m); rows[3].Selection.Firing != "older" {
		t.Fatalf("firings are not newest first: %+v", rows)
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on a firing returned no command")
	}
	request, ok := m.effects.Main.Active.Request.(uilaunch.Request)
	if !ok || request.Plan == nil || request.Plan.Command != "tail -n 200 -F '"+logPath+"'" || request.Name != "log-Nightly-report-abcd" {
		t.Fatalf("queued %+v", m.effects.Main.Active)
	}
}
