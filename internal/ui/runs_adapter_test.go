package ui

import (
	"strings"
	"testing"
	"time"

	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	"github.com/YoanWai/agent-manager/internal/ui/runs"
	uilaunch "github.com/YoanWai/agent-manager/internal/ui/spawn/launch"
	"github.com/charmbracelet/x/ansi"
)

func testRuns() []runs.Record {
	return []runs.Record{
		{Version: 1, ID: "r1", TicketRef: "jira:ABC-1", Status: runs.StatusNeedsHuman, Branch: "fractory/abc-1", PRURL: "https://example.test/pr/1",
			LogPath: "/tmp/fractory logs/r1.log", UpdatedAt: time.Now(), Result: &runs.Result{Outcome: "needs-human", Question: "Which table owns the column?"}},
		{Version: 1, ID: "r2", TicketRef: "jira:ABC-2", Status: runs.StatusDone, Branch: "fractory/abc-2", LogPath: "/tmp/r2.log", UpdatedAt: time.Now()},
	}
}

func loadRuns(t *testing.T, m *Model, list []runs.Record) {
	t.Helper()
	m.applyTestMsg(t, runsLoadedMsg{dir: "/registry", list: list})
}

func selectRun(t *testing.T, m *Model, id string) {
	t.Helper()
	if !m.rail.Focus(uirail.Selection{Kind: uirail.RunRow, Run: id}) {
		t.Fatalf("no row for run %s in %+v", id, m.rail.Rows())
	}
}

func TestRunsSectionIsHiddenWithoutRuns(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, nil)
	for _, row := range m.rail.Rows() {
		if row.Kind == uirail.RunsRow || row.Kind == uirail.RunRow {
			t.Fatalf("empty registry listed %+v", row)
		}
	}
}

func TestRunsLoadListsRunsAfterTheSessionTree(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, testRuns())
	var kinds []uirail.RowKind
	for _, row := range m.rail.Rows() {
		kinds = append(kinds, row.Kind)
	}
	if n := len(kinds); n < 3 || kinds[n-3] != uirail.RunsRow || kinds[n-2] != uirail.RunRow || kinds[n-1] != uirail.RunRow {
		t.Fatalf("rows = %v; want the Runs header and both Runs last", kinds)
	}
}

func TestRunDetailShowsBranchPRAndTheQuestion(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, testRuns())
	selectRun(t, m, "r1")
	lines, ok := m.runContentLines(80)
	if !ok {
		t.Fatal("no detail for a selected run")
	}
	var text []string
	for _, line := range lines {
		text = append(text, ansi.Strip(line.Text))
	}
	joined := strings.Join(text, "\n")
	for _, want := range []string{"jira:ABC-1", "needs-human", "fractory/abc-1", "https://example.test/pr/1", "Which table owns the column?"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("detail lacks %q:\n%s", want, joined)
		}
	}
	selectRun(t, m, "r2")
	lines, _ = m.runContentLines(80)
	for _, line := range lines {
		if strings.Contains(ansi.Strip(line.Text), "Needs a human") {
			t.Fatal("a done run shows the needs-human question")
		}
	}
}

func TestOpeningARunQueuesALogTail(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, testRuns())
	selectRun(t, m, "r1")
	cmd, taken := m.runIntent(uirail.Intent{Kind: uirail.Attach, Target: m.railSelection()})
	if !taken || cmd == nil {
		t.Fatalf("attach on a run: taken %v, cmd %v", taken, cmd)
	}
	request, ok := m.effects.Main.Active.Request.(uilaunch.Request)
	if !ok || request.Kind != uilaunch.Shell || request.Plan == nil {
		t.Fatalf("queued %+v; want a shell spawn with a plan", m.effects.Main.Active)
	}
	if want := "tail -n 200 -F '/tmp/fractory logs/r1.log'"; request.Plan.Command != want {
		t.Fatalf("command = %q, want %q", request.Plan.Command, want)
	}
	if request.Dir != "/tmp/fractory logs" {
		t.Fatalf("dir = %q", request.Dir)
	}
}

func TestRunsRefuseTheKeysThatChangeASession(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, testRuns())
	selectRun(t, m, "r1")
	for _, kind := range []uirail.ActionKind{uirail.Kill, uirail.Delete, uirail.Archive, uirail.RenameAction, uirail.Fork} {
		m.clearErr()
		if _, taken := m.runIntent(uirail.Intent{Kind: kind, Target: m.railSelection()}); !taken || m.errBar.text == "" {
			t.Fatalf("intent %d on a run: taken %v, status %q", kind, taken, m.errBar.text)
		}
	}
	if _, taken := m.runIntent(uirail.Intent{Kind: uirail.Quit, Target: m.railSelection()}); taken {
		t.Fatal("quit must stay with the manager")
	}
	if len(m.workspace.sessions) != 0 || m.effects.Main.Active != nil {
		t.Fatal("a refused key changed the workspace")
	}
}

func TestRunsLoadedWithTheSameListKeepsTheCursor(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, testRuns())
	selectRun(t, m, "r2")
	loadRuns(t, m, testRuns())
	if got := m.railSelection(); got.Kind != uirail.RunRow || got.Run != "r2" {
		t.Fatalf("selection = %+v after a poll with nothing new", got)
	}
}

func TestRunLegendNamesTheLogKey(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, testRuns())
	selectRun(t, m, "r1")
	legend, ok := m.runLegend()
	if !ok || legend.Title != "Run" || len(legend.Pairs) != 1 || legend.Pairs[0][1] != "open log" {
		t.Fatalf("legend = %+v, %v", legend, ok)
	}
}
