package ui

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/git"
	"github.com/YoanWai/agent-manager/internal/status"
	"github.com/YoanWai/agent-manager/internal/store"
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
	selectRun(t, m, "r2")
	legend, ok := m.runLegend()
	if !ok || legend.Title != "Run" || len(legend.Pairs) != 1 || legend.Pairs[0][1] != "open log" {
		t.Fatalf("legend = %+v, %v", legend, ok)
	}
}

func TestReviewingARunOpensItsWorktreeAgainstTheBase(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	list := testRuns()
	list[0].WorktreePath, list[0].Base = t.TempDir(), "develop"
	loadRuns(t, m, list)
	selectRun(t, m, "r1")
	cmd, taken := m.runIntent(uirail.Intent{Kind: uirail.OpenReview, Target: m.railSelection()})
	if !taken || cmd == nil || m.base != baseReview || !m.review.Active() {
		t.Fatalf("review did not open: taken %v, cmd %v, base %v", taken, cmd, m.base)
	}
	if m.review.SessionID() != "run:r1" || m.review.Scope() != git.ScopeBranch {
		t.Fatalf("target %q scope %v", m.review.SessionID(), m.review.Scope())
	}
	sess, ok := m.diffSession()
	if !ok || sess.ID != "run:r1" || sess.Cwd != list[0].WorktreePath {
		t.Fatalf("diffSession = %+v, %v", sess, ok)
	}
	if got := m.reviewGroupBase("run:r1"); got != "develop" {
		t.Fatalf("base = %q", got)
	}
	if got := m.runsDirFor("run:r1"); got != "/registry" {
		t.Fatalf("runs dir = %q", got)
	}
	if got := m.runsDirFor("some-session"); got != "" {
		t.Fatalf("a session got runs dir %q", got)
	}
}

func TestReviewingARunWithoutAWorktreeSaysSo(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, testRuns())
	selectRun(t, m, "r2")
	if cmd, taken := m.runIntent(uirail.Intent{Kind: uirail.OpenReview, Target: m.railSelection()}); !taken || cmd != nil || m.errBar.text == "" || m.review.Active() {
		t.Fatalf("taken %v, cmd %v, status %q", taken, cmd, m.errBar.text)
	}
}

func TestRunDetailShowsModeFlowAgentsAndRules(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	list := testRuns()
	list[0].Mode, list[0].Flow, list[0].Agents = runs.ModeWork, "standard", []string{"claude@work"}
	loadRuns(t, m, list)
	text := func(id string) string {
		selectRun(t, m, id)
		lines, _ := m.runContentLines(80)
		var out []string
		for _, line := range lines {
			out = append(out, ansi.Strip(line.Text))
		}
		return strings.Join(out, "\n")
	}
	work := text("r1")
	for _, want := range []string{"Mode", "work", "standard", "claude@work", "Workflow rules", "PR Gate: human merges"} {
		if !strings.Contains(work, want) {
			t.Fatalf("work detail lacks %q:\n%s", want, work)
		}
	}
	if unknown := text("r2"); !strings.Contains(unknown, "unknown") || strings.Contains(unknown, "PR Gate") {
		t.Fatalf("a Run with no mode must say unknown:\n%s", unknown)
	}
}

func ticketRuns() []runs.Record {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	mk := func(id, key, kind, status string, at time.Time) runs.Record {
		return runs.Record{Version: 1, ID: id, Key: key, TicketRef: "jira:ABC-1", Trigger: runs.Trigger{Kind: kind}, Status: status,
			LogPath: "/tmp/" + id + ".log", CreatedAt: at, UpdatedAt: at}
	}
	return []runs.Record{
		mk("r3", "jira:ABC-1:rerun:r2", "manual", runs.StatusFailed, t0.Add(2*time.Hour)),
		mk("r2", "jira:ABC-1:rerun:r1", "manual", runs.StatusFailed, t0.Add(time.Hour)),
		mk("r1", "jira:ABC-1", "ticket-ready", runs.StatusFailed, t0),
	}
}

func TestRunsListsOneRowPerTicketWithTheLatestRun(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, ticketRuns())
	var rows []uirail.Selection
	for _, row := range m.rail.Rows() {
		if row.Kind == uirail.RunRow {
			rows = append(rows, row.Selection)
		}
	}
	if len(rows) != 1 || rows[0].Run != "r3" {
		t.Fatalf("run rows = %+v; want only the latest Run r3", rows)
	}
	selectRun(t, m, "r3")
	lines, _ := m.runContentLines(100)
	var text []string
	for _, line := range lines {
		text = append(text, ansi.Strip(line.Text))
	}
	joined := strings.Join(text, "\n")
	for _, want := range []string{"ticket-ready", "manual", "/tmp/r1.log", "/tmp/r2.log", "/tmp/r3.log"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("history lacks %q:\n%s", want, joined)
		}
	}
	if strings.Index(joined, "/tmp/r3.log") > strings.Index(joined, "/tmp/r1.log") {
		t.Fatalf("history is not newest first:\n%s", joined)
	}
}

func TestRerunKeyWritesOneRequestAndShowsPending(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	dir := t.TempDir()
	m.applyTestMsg(t, runsLoadedMsg{dir: dir, list: ticketRuns()})
	selectRun(t, m, "r3")
	legend, _ := m.runLegend()
	if len(legend.Pairs) != 2 || legend.Pairs[1][1] != "re-run" {
		t.Fatalf("legend = %+v; want the re-run hint for a failed Run", legend)
	}
	intent := uirail.Intent{Kind: uirail.Restart, Target: m.railSelection()}
	if _, taken := m.runIntent(intent); !taken {
		t.Fatal("R was not taken by the Runs view")
	}
	path := runs.RerunPath(dir, "r3")
	first, err := os.ReadFile(path)
	if err != nil || !strings.HasPrefix(string(first), `{"version":1,"requestedAt":"`) {
		t.Fatalf("request = %q, %v", first, err)
	}
	m.runIntent(intent)
	if again, _ := os.ReadFile(path); string(again) != string(first) {
		t.Fatal("a second press changed the request")
	}
	if !m.workspace.runs.requested["r3"] {
		t.Fatal("the row does not show the pending Re-run")
	}
}

func TestRerunIsOfferedOnlyForFailedOrNeedsHuman(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	dir := t.TempDir()
	m.applyTestMsg(t, runsLoadedMsg{dir: dir, list: testRuns()})
	selectRun(t, m, "r2") // done
	if legend, _ := m.runLegend(); len(legend.Pairs) != 1 {
		t.Fatalf("a done Run's legend = %+v", legend)
	}
	m.runIntent(uirail.Intent{Kind: uirail.Restart, Target: m.railSelection()})
	if entries, _ := os.ReadDir(dir); len(entries) != 0 || m.errBar.text == "" {
		t.Fatalf("a done Run was re-run: %v, status %q", entries, m.errBar.text)
	}
	selectRun(t, m, "r1") // needs-human
	if legend, _ := m.runLegend(); len(legend.Pairs) != 2 {
		t.Fatalf("a needs-human Run's legend = %+v", legend)
	}
}

func TestHeldLineShowsAboveTheRunsDetail(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	host := &runs.HostState{Held: true, Reason: "Docker memory 0.8 GB free < 1 GB", HeldSince: time.Now().Add(-42*time.Minute - time.Second)}
	m.applyTestMsg(t, runsLoadedMsg{dir: "/registry", list: testRuns(), host: host})
	selectRun(t, m, "r2")
	lines, _ := m.runContentLines(100)
	if len(lines) == 0 || !strings.Contains(ansi.Strip(lines[0].Text), "Held 42 min: Docker memory 0.8 GB free < 1 GB") {
		t.Fatalf("first line = %+v", lines)
	}
	m.applyTestMsg(t, runsLoadedMsg{dir: "/registry", list: testRuns()})
	lines, _ = m.runContentLines(100)
	if strings.Contains(ansi.Strip(lines[0].Text), "Held") {
		t.Fatal("a Held line without host.json")
	}
}

func TestOpeningARunLogTwiceReusesTheTab(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, testRuns())
	run := testRuns()[0]
	if got := runLogName(run); got != "log-ABC-1-r1" {
		t.Fatalf("name = %q", got)
	}
	selectRun(t, m, "r1")
	m.openRunLog(run)
	request, ok := m.effects.Main.Active.Request.(uilaunch.Request)
	if !ok || request.Name != "log-ABC-1-r1" {
		t.Fatalf("spawn = %+v", m.effects.Main.Active)
	}
	m.effects.Main.Active = nil
	m.workspace.sessions = append(m.workspace.sessions, store.Session{ID: "tab", Name: "log-ABC-1-r1", Tool: "zsh", Status: status.Idle})
	m.rebuildRows()
	selectRun(t, m, "r1")
	m.openRunLog(run)
	if m.effects.Main.Active != nil && m.effects.Main.Active.Request != nil {
		if _, spawned := m.effects.Main.Active.Request.(uilaunch.Request); spawned {
			t.Fatal("a second tab was spawned for the same Run")
		}
	}
	if sel, _ := m.rail.SelectedSession(); sel != "tab" {
		t.Fatalf("selected session = %q, want the existing tab", sel)
	}
}

func finishedRuns() []runs.Record {
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	mk := func(id, ticket, status string, cleaned bool, at time.Time) runs.Record {
		record := runs.Record{Version: 1, ID: id, Key: ticket, TicketRef: ticket, Status: status, LogPath: "/tmp/" + id + ".log", CreatedAt: at, UpdatedAt: at}
		if cleaned {
			record.CleanedAt = at.Add(time.Hour)
		}
		return record
	}
	return []runs.Record{
		mk("open", "jira:OPEN-1", runs.StatusDone, false, t0.Add(3*time.Hour)),
		mk("merged", "jira:MERGED-1", runs.StatusDone, true, t0.Add(2*time.Hour)),
		mk("merged-old", "jira:MERGED-1", runs.StatusFailed, false, t0.Add(time.Hour)),
		mk("gone", "jira:GONE-1", runs.StatusDone, true, t0),
	}
}

func listedRuns(m *Model) []string {
	var ids []string
	for _, row := range m.rail.Rows() {
		if row.Kind == uirail.RunRow {
			ids = append(ids, row.Selection.Run)
		}
	}
	return ids
}

func TestFinishedTicketsAreHiddenUntilH(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, finishedRuns())
	if got := listedRuns(m); strings.Join(got, ",") != "open" {
		t.Fatalf("listed %v; want only the Ticket whose PR is open", got)
	}
	selectRun(t, m, "open")
	toggle := uirail.Intent{Kind: uirail.ToggleFinishedRuns, Target: m.railSelection()}
	if _, taken := m.runIntent(toggle); !taken {
		t.Fatal("H was not taken by the Runs view")
	}
	if got := listedRuns(m); strings.Join(got, ",") != "open,merged,gone" {
		t.Fatalf("listed %v after H; want every Ticket", got)
	}
	loadRuns(t, m, finishedRuns())
	if got := listedRuns(m); len(got) != 3 {
		t.Fatalf("a poll reset the toggle: %v", got)
	}
	selectRun(t, m, "merged")
	lines, _ := m.runContentLines(100)
	var text []string
	for _, line := range lines {
		text = append(text, ansi.Strip(line.Text))
	}
	if joined := strings.Join(text, "\n"); !strings.Contains(joined, "/tmp/merged-old.log") {
		t.Fatalf("a finished Ticket's history lost a Run:\n%s", joined)
	}
	m.runIntent(toggle)
	if got := listedRuns(m); strings.Join(got, ",") != "open" {
		t.Fatalf("listed %v after a second H", got)
	}
}

func TestRunsHeaderAndLegendNameTheFinishedTickets(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, finishedRuns())
	if view := ansi.Strip(preparedView(m)); !strings.Contains(view, "Runs (1, 2 finished)") {
		t.Fatalf("view lacks the header count:\n%s", view)
	}
	m.rail.Focus(uirail.Selection{Kind: uirail.RunsRow})
	legend, _ := m.runLegend()
	if !slices.Contains(legend.Pairs, [2]string{"H", "show finished"}) {
		t.Fatalf("header legend = %+v", legend)
	}
	preparedView(m)
	if footer := ansi.Strip(m.viewFooter()); !strings.Contains(footer, "show finished") {
		t.Fatalf("footer lacks the hint:\n%s", footer)
	}
	m.runIntent(uirail.Intent{Kind: uirail.ToggleFinishedRuns, Target: m.railSelection()})
	selectRun(t, m, "gone")
	legend, _ = m.runLegend()
	if !slices.Contains(legend.Pairs, [2]string{"H", "hide finished"}) {
		t.Fatalf("run legend = %+v", legend)
	}
	if view := ansi.Strip(preparedView(m)); !strings.Contains(view, "Runs (3, 2 finished)") {
		t.Fatalf("view lacks the header count while shown:\n%s", view)
	}

	plain := buildModel(t)
	loadRuns(t, plain, testRuns())
	selectRun(t, plain, "r2")
	if legend, _ := plain.runLegend(); slices.ContainsFunc(legend.Pairs, func(pair [2]string) bool { return strings.Contains(pair[1], "finished") }) {
		t.Fatalf("legend offers H with nothing finished: %+v", legend)
	}
	if view := ansi.Strip(preparedView(plain)); !strings.Contains(view, "Runs (2)") || strings.Contains(view, "finished)") {
		t.Fatalf("header without finished Tickets:\n%s", view)
	}
}
