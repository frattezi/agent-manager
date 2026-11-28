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
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	"github.com/YoanWai/agent-manager/internal/ui/runs"
	uilaunch "github.com/YoanWai/agent-manager/internal/ui/spawn/launch"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// A yearly schedule keeps a check from firing the fixtures while a test runs.
func viewAutomationDoc(id, name, mode string) []byte {
	return []byte(`{"version":1,"id":"` + id + `","name":"` + name + `","kind":"script","repoPath":"/src/repo","cron":"0 7 1 1 *","timezone":"UTC","mode":"` + mode + `","cli":"","flags":[],"entryPoint":"scripts/` + id + `.sh","keepRuns":3}`)
}

func addViewAutomation(t *testing.T, m *Model, id, name, mode string, enabled bool, created time.Time) {
	t.Helper()
	if err := m.services.store.AddAutomation(store.Automation{ID: id, Name: name, Config: viewAutomationDoc(id, name, mode), Enabled: enabled}, created); err != nil {
		t.Fatal(err)
	}
}

func writeFiringRecord(t *testing.T, home, id, automation, status, outcome string, firedFor, started, updated time.Time, logPath string) {
	t.Helper()
	dir := filepath.Join(home, "automations", automation)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	result := "null"
	if outcome != "" {
		result = `{"outcome":"` + outcome + `","summary":"wrote the report"}`
	}
	stamp := func(at time.Time) string {
		if at.IsZero() {
			return `"0001-01-01T00:00:00Z"`
		}
		return `"` + at.UTC().Format(time.RFC3339) + `"`
	}
	body := `{"version":1,"id":"` + id + `","automationId":"` + automation + `","firedFor":` + stamp(firedFor) + `,"status":"` + status +
		`","startedAt":` + stamp(started) + `,"updatedAt":` + stamp(updated) + `,"logPath":"` + logPath + `","result":` + result + `,"outputPath":"/out/` + id + `"}`
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func checkAutomations(t *testing.T, m *Model) {
	t.Helper()
	m.applyTestMsg(t, automationsCheckNow())
	m.drainEffects(t)
}

// automationFixtures stores a weekly Automation with two firings, a
// disabled one that last failed and one that never fired.
func automationFixtures(t *testing.T) (*Model, string, time.Time) {
	t.Helper()
	home := fractoryHome(t)
	m := buildModel(t)
	created := time.Now().Add(-48 * time.Hour)
	addViewAutomation(t, m, "report", "weekly report", "work", true, created)
	addViewAutomation(t, m, "sweep", "monday sweep", "personal", false, created.Add(time.Second))
	addViewAutomation(t, m, "fresh", "fresh chore", "work", true, created.Add(2*time.Second))
	older := time.Date(2026, 11, 16, 7, 0, 0, 0, time.UTC)
	newer := older.Add(7 * 24 * time.Hour)
	writeFiringRecord(t, home, "run-old", "report", "done", "done", older, older.Add(time.Minute), older.Add(3*time.Minute), "/logs/report old.log")
	writeFiringRecord(t, home, "run-new", "report", "running", "", newer, newer.Add(time.Minute), newer.Add(2*time.Minute), "/logs/report-new.log")
	writeFiringRecord(t, home, "sweep-1", "sweep", "failed", "failed", older, older, older.Add(90*time.Second), "/logs/sweep.log")
	return m, home, newer
}

func automationRowsOf(m *Model) []uirail.Row {
	var out []uirail.Row
	for _, row := range m.rail.Rows() {
		if row.Kind == uirail.AutomationsRow || row.Kind == uirail.AutomationRow || row.Kind == uirail.FiringRow {
			out = append(out, row)
		}
	}
	return out
}

func detailText(t *testing.T, m *Model) string {
	t.Helper()
	lines, ok := m.automationContentLines(100)
	if !ok {
		t.Fatal("no detail for the Automations section")
	}
	var text []string
	for _, line := range lines {
		text = append(text, ansi.Strip(line.Text))
	}
	return strings.Join(text, "\n")
}

func railLineOf(t *testing.T, m *Model, selection uirail.Selection) int {
	t.Helper()
	preparedView(m)
	for i, line := range m.layout.displayedRail.Lines {
		if line.HitOK && line.Hit == selection {
			return i
		}
	}
	t.Fatalf("test setup: %+v painted no rail line", selection)
	return -1
}

func doubleClick(t *testing.T, m *Model, selection uirail.Selection) tea.Cmd {
	t.Helper()
	y0, _ := m.bodyYRange()
	press := tea.MouseMsg{X: 8, Y: y0 + railLineOf(t, m, selection), Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	m.handleMouse(press)
	m.drainEffects(t)
	_, cmd := m.handleMouse(press)
	return cmd
}

func TestAutomationsSectionListsEveryStoredAutomation(t *testing.T) {
	m, _, _ := automationFixtures(t)
	checkAutomations(t, m)
	rows := automationRowsOf(m)
	var names []string
	for _, row := range rows {
		names = append(names, row.Name)
	}
	if want := []string{"automations", "weekly report", "monday sweep", "fresh chore"}; !slices.Equal(names, want) {
		t.Fatalf("Automations rows = %v; want %v", names, want)
	}
	view := ansi.Strip(preparedView(m))
	for _, want := range []string{"Automations (3)", "weekly report", "running · next ", "failed · disabled", "never fired · next "} {
		if !strings.Contains(view, want) {
			t.Fatalf("rail lacks %q:\n%s", want, view)
		}
	}
	for name, badge := range map[string]string{"weekly report": "W", "monday sweep": "P", "fresh chore": "W"} {
		line := view[strings.Index(view, name):]
		line = line[:strings.Index(line, "\n")]
		if fields := strings.Fields(strings.TrimSuffix(strings.TrimSpace(line), "▐")); !slices.Contains(fields, badge) {
			t.Fatalf("%s row %q lacks its Mode %s", name, line, badge)
		}
	}
	if strings.Contains(m.errBar.text, "Automation") {
		t.Fatalf("status = %q", m.errBar.text)
	}
}

func TestAutomationDetailListsRecentFiringsNewestFirst(t *testing.T) {
	m, _, newer := automationFixtures(t)
	checkAutomations(t, m)
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "report"})
	text := detailText(t, m)
	for _, want := range []string{"weekly report", "enabled", "0 7 1 1 * (UTC)", "/src/repo", "scripts/report.sh", "Recent firings, newest first"} {
		if !strings.Contains(text, want) {
			t.Fatalf("detail lacks %q:\n%s", want, text)
		}
	}
	newLine := firingClock(newer) + " running "
	oldLine := firingClock(newer.Add(-7*24*time.Hour)) + " done 2m0s"
	first, second := strings.Index(text, newLine), strings.Index(text, oldLine)
	if first < 0 || second < 0 || first > second {
		t.Fatalf("firings not listed newest first with outcome and duration (%d, %d):\n%s", first, second, text)
	}
	if !strings.Contains(text[first:second], "so far") {
		t.Fatalf("a running firing does not say it is still going:\n%s", text)
	}

	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "fresh"})
	if text := detailText(t, m); !strings.Contains(text, "Never fired") {
		t.Fatalf("a fresh Automation's detail:\n%s", text)
	}
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "sweep"})
	if text := detailText(t, m); !strings.Contains(text, "disabled") || strings.Contains(text, "Next") {
		t.Fatalf("a disabled Automation's detail:\n%s", text)
	}
}

func TestHeldLineShowsAboveTheAutomationDetail(t *testing.T) {
	m, _, _ := automationFixtures(t)
	checkAutomations(t, m)
	host := &runs.HostState{Held: true, Reason: "Docker memory low", HeldSince: time.Now().Add(-5*time.Minute - time.Second)}
	m.applyTestMsg(t, runsLoadedMsg{dir: "/registry", host: host})
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "report"})
	if text := detailText(t, m); !strings.HasPrefix(strings.TrimSpace(text), "Held 5 min: Docker memory low") {
		t.Fatalf("detail does not open on the Held line:\n%s", text)
	}
}

func TestAutomationsWithoutRecordsShowAnEmptySection(t *testing.T) {
	home := fractoryHome(t)
	m := buildModel(t)
	checkAutomations(t, m)
	if rows := automationRowsOf(m); len(rows) != 0 {
		t.Fatalf("no Automations listed %+v", rows)
	}
	addViewAutomation(t, m, "report", "weekly report", "work", true, time.Now().Add(-time.Hour))
	if err := os.MkdirAll(filepath.Join(home, "automations"), 0o755); err != nil {
		t.Fatal(err)
	}
	checkAutomations(t, m)
	if rows := automationRowsOf(m); len(rows) != 2 {
		t.Fatalf("rows = %+v; want the header and the Automation", rows)
	}
	if view := ansi.Strip(preparedView(m)); !strings.Contains(view, "never fired") {
		t.Fatalf("rail:\n%s", view)
	}
	if m.errBar.text != "" {
		t.Fatalf("an empty records directory reported %q", m.errBar.text)
	}
	if err := os.RemoveAll(filepath.Join(home, "automations")); err != nil {
		t.Fatal(err)
	}
	checkAutomations(t, m)
	if rows := automationRowsOf(m); len(rows) != 2 || m.errBar.text != "" {
		t.Fatalf("an absent records directory: rows %+v, status %q", rows, m.errBar.text)
	}
}

func TestAutomationsRefreshAsRecordsChange(t *testing.T) {
	m, home, newer := automationFixtures(t)
	checkAutomations(t, m)
	writeFiringRecord(t, home, "run-new", "report", "done", "failed", newer, newer.Add(time.Minute), newer.Add(4*time.Minute), "/logs/report-new.log")
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "report"})
	checkAutomations(t, m)
	if view := ansi.Strip(preparedView(m)); !strings.Contains(view, "failed · next ") {
		t.Fatalf("the row did not take the new record:\n%s", view)
	}
	if text := detailText(t, m); !strings.Contains(text, firingClock(newer)+" failed 3m0s") {
		t.Fatalf("the detail did not take the new record:\n%s", text)
	}
}

func TestEnterOnAnAutomationListsItsFiringsAndEnterOnAFiringOpensItsLog(t *testing.T) {
	m, _, newer := automationFixtures(t)
	checkAutomations(t, m)
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "report"})
	legend, _ := m.automationLegend()
	if !slices.Contains(legend.Pairs, [2]string{m.listGlyph(keybind.Open), "show firings"}) || !slices.Contains(legend.Pairs, [2]string{"click ▸", "show firings"}) {
		t.Fatalf("folded Automation legend = %+v", legend)
	}
	m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyEnter})
	m.drainEffects(t)
	var firings []string
	for _, row := range automationRowsOf(m) {
		if row.Kind == uirail.FiringRow {
			firings = append(firings, row.Firing)
		}
	}
	if !slices.Equal(firings, []string{"run-new", "run-old"}) {
		t.Fatalf("firing rows = %v; want both, newest first", firings)
	}
	if footer := ansi.Strip(m.viewFooter()); !strings.Contains(footer, "hide firings") {
		t.Fatalf("footer lacks the fold key:\n%s", footer)
	}

	m.rail.Focus(uirail.Selection{Kind: uirail.FiringRow, Automation: "report", Firing: "run-old"})
	if footer := ansi.Strip(m.viewFooter()); !strings.Contains(footer, "open log") {
		t.Fatalf("footer lacks the log key:\n%s", footer)
	}
	if text := detailText(t, m); !strings.Contains(text, "wrote the report") || !strings.Contains(text, "/out/run-old") {
		t.Fatalf("firing detail:\n%s", text)
	}
	m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyEnter})
	request, ok := m.effects.Main.Active.Request.(uilaunch.Request)
	if !ok || request.Kind != uilaunch.Shell || request.Plan == nil {
		t.Fatalf("queued %+v; want a shell spawn with a plan", m.effects.Main.Active)
	}
	if want := "tail -n 200 -F '/logs/report old.log'"; request.Plan.Command != want {
		t.Fatalf("command = %q; want %q", request.Plan.Command, want)
	}
	if request.Name != "log-report-"+newer.Add(-7*24*time.Hour).Format("20060102T1504") || request.Dir != "/logs" {
		t.Fatalf("tab = %q in %q", request.Name, request.Dir)
	}
}

func TestClicksFoldAnAutomationAndOpenAFiringsLog(t *testing.T) {
	m, _, _ := automationFixtures(t)
	checkAutomations(t, m)
	automation := uirail.Selection{Kind: uirail.AutomationRow, Automation: "report"}
	doubleClick(t, m, automation)
	m.drainEffects(t)
	if !slices.ContainsFunc(automationRowsOf(m), func(row uirail.Row) bool { return row.Kind == uirail.FiringRow }) {
		t.Fatalf("a double click left the firings folded: %+v", automationRowsOf(m))
	}
	doubleClick(t, m, uirail.Selection{Kind: uirail.FiringRow, Automation: "report", Firing: "run-new"})
	request, ok := m.effects.Main.Active.Request.(uilaunch.Request)
	if !ok || request.Plan == nil || request.Plan.Command != "tail -n 200 -F '/logs/report-new.log'" {
		t.Fatalf("queued %q; want the newest firing's log followed", request.Plan.Command)
	}
}

func TestAttachOnAnAutomationOpensItsNewestLogOrSaysItNeverFired(t *testing.T) {
	m, _, _ := automationFixtures(t)
	checkAutomations(t, m)
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "report"})
	if _, taken := m.automationIntent(uirail.Intent{Kind: uirail.Attach, Target: m.railSelection()}); !taken {
		t.Fatal("attach on an Automation was not taken")
	}
	if request, ok := m.effects.Main.Active.Request.(uilaunch.Request); !ok || request.Plan == nil || !strings.HasSuffix(request.Plan.Command, "'/logs/report-new.log'") {
		t.Fatalf("queued %+v; want the newest firing's log", m.effects.Main.Active)
	}
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "fresh"})
	m.automationIntent(uirail.Intent{Kind: uirail.Attach, Target: m.railSelection()})
	if !strings.Contains(m.errBar.text, "never fired") {
		t.Fatalf("status = %q", m.errBar.text)
	}
	m.automationIntent(uirail.Intent{Kind: uirail.Kill, Target: m.railSelection()})
	if !strings.Contains(m.errBar.text, "r edit") {
		t.Fatalf("a kill on an Automation: status = %q", m.errBar.text)
	}
}
