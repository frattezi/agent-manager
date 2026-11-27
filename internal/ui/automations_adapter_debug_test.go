package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/store"
	uieffects "github.com/YoanWai/agent-manager/internal/ui/effects"
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	uilaunch "github.com/YoanWai/agent-manager/internal/ui/spawn/launch"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func queuedMainEffects(m *Model) []uieffects.Request {
	var out []uieffects.Request
	for _, job := range append([]*uieffects.Job{m.effects.Main.Active}, m.effects.Main.Pending...) {
		if job != nil {
			out = append(out, job.Request)
		}
	}
	return out
}

func queuedDebugSpawns(m *Model) []uilaunch.Request {
	var out []uilaunch.Request
	for _, request := range queuedMainEffects(m) {
		if spawn, ok := request.(uilaunch.Request); ok && spawn.Kind == uilaunch.Debug {
			out = append(out, spawn)
		}
	}
	return out
}

// showFirings unfolds an Automation, so its firing rows can take the cursor.
func showFirings(t *testing.T, m *Model, id string) {
	t.Helper()
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: id})
	m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyEnter})
	m.drainEffects(t)
}

func agentAutomationDoc(id, name, repo, tool string) []byte {
	return []byte(`{"version":1,"id":"` + id + `","name":"` + name + `","kind":"agent","repoPath":"` + repo + `","cron":"0 7 1 1 *","timezone":"UTC","mode":"work","tool":"` + tool + `","flags":["--yolo"],"entry":"prompts/` + id + `.md","retention":3}`)
}

func TestDebugSessionOnAFinishedFiringSpawnsUnderANewProjectGroup(t *testing.T) {
	m, _, older := automationFixtures(t)
	checkAutomations(t, m)
	showFirings(t, m, "report")
	m.rail.Focus(uirail.Selection{Kind: uirail.FiringRow, Automation: "report", Firing: "run-old"})
	if footer := ansi.Strip(m.viewFooter()); !strings.Contains(footer, "D debug session") {
		t.Fatalf("footer lacks the Debug Session key:\n%s", footer)
	}
	m.applyTestMsg(t, runeKey("D"))
	queued := queuedMainEffects(m)
	if len(queued) != 2 {
		t.Fatalf("queued %+v; want the group, then the spawn", queued)
	}
	group, ok := queued[0].(uilaunch.GroupRequest)
	if !ok || group.Path != "repo" || group.RawDir != "/src/repo" {
		t.Fatalf("first effect = %+v; want the repo group created on /src/repo", queued[0])
	}
	spawns := queuedDebugSpawns(m)
	if len(spawns) != 1 {
		t.Fatalf("queued %+v; want one Debug Session", queued)
	}
	spawn := spawns[0]
	firedFor := older.Add(-7 * 24 * time.Hour)
	if spawn.Group != "repo" || spawn.RawDir != "/src/repo" || spawn.WantWorktree || spawn.AutoNamed {
		t.Fatalf("spawn placed %+v", spawn)
	}
	if want := "debug-weekly-report-" + firedFor.Format("20060102-1504"); spawn.Name != want {
		t.Fatalf("name = %q; want %q", spawn.Name, want)
	}
	for _, want := range []string{`"weekly report"`, firedFor.Format(time.RFC3339), "outcome was done", `"/logs/report old.log"`, `"/wt/run-old"`} {
		if !strings.Contains(spawn.Prompt, want) {
			t.Fatalf("prompt lacks %q:\n%s", want, spawn.Prompt)
		}
	}
	if strings.Contains(spawn.Prompt, "wrote the report") {
		t.Fatalf("prompt pastes the firing's summary:\n%s", spawn.Prompt)
	}
	// A Script Automation records no CLI: the picker's default runs it.
	if tools, index := m.cachedDefaultToolSelection(); spawn.ToolName != tools[index] {
		t.Fatalf("tool = %q; want the default %q", spawn.ToolName, tools[index])
	}
}

func TestDebugSessionGoesUnderTheRepositorysExistingGroupWithItsCLI(t *testing.T) {
	home := fractoryHome(t)
	m := buildModel(t)
	tool := m.services.cfg.AgentToolNames()[len(m.services.cfg.AgentToolNames())-1]
	if err := m.services.store.AddAutomation(store.Automation{ID: "recon", Name: "reconcile", Config: agentAutomationDoc("recon", "reconcile", "/src/books", tool), Enabled: true}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := m.services.store.CreateGroup("clients/books", "/src/books"); err != nil {
		t.Fatal(err)
	}
	m.applyCmd(t, m.refreshCmd())
	at := time.Date(2026, 11, 23, 7, 0, 0, 0, time.UTC)
	writeFiringRecord(t, home, "recon-1", "recon", "failed", "", at, at, at.Add(time.Minute), "/logs/recon.log")
	checkAutomations(t, m)
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "recon"})
	m.applyTestMsg(t, runeKey("D"))
	if spawns := queuedDebugSpawns(m); len(spawns) != 0 {
		t.Fatalf("D on an Automation row spawned %+v", spawns)
	}
	showFirings(t, m, "recon")
	m.rail.Focus(uirail.Selection{Kind: uirail.FiringRow, Automation: "recon", Firing: "recon-1"})
	m.applyTestMsg(t, runeKey("D"))
	for _, request := range queuedMainEffects(m) {
		if _, ok := request.(uilaunch.GroupRequest); ok {
			t.Fatalf("a repository with a group got another: %+v", request)
		}
	}
	spawns := queuedDebugSpawns(m)
	if len(spawns) != 1 || spawns[0].Group != "clients/books" || spawns[0].ToolName != tool || spawns[0].Name != "debug-reconcile-20261123-0700" {
		t.Fatalf("queued %+v; want reconcile's Debug Session under clients/books on %s", spawns, tool)
	}
	if strings.Contains(spawns[0].Prompt, "--yolo") {
		t.Fatalf("the Debug Session took the Automation's unattended flags:\n%s", spawns[0].Prompt)
	}
}

func TestUnfinishedFiringOpensNoDebugSessionAndSaysWhy(t *testing.T) {
	m, _, _ := automationFixtures(t)
	checkAutomations(t, m)
	showFirings(t, m, "report")
	m.rail.Focus(uirail.Selection{Kind: uirail.FiringRow, Automation: "report", Firing: "run-new"})
	if footer := ansi.Strip(m.viewFooter()); strings.Contains(footer, "debug session") {
		t.Fatalf("a running firing offers a Debug Session:\n%s", footer)
	}
	m.applyTestMsg(t, runeKey("D"))
	if queued := queuedMainEffects(m); len(queued) != 0 {
		t.Fatalf("a running firing queued %+v", queued)
	}
	if !strings.Contains(m.errBar.text, "not finished") {
		t.Fatalf("status = %q; want the reason", m.errBar.text)
	}
}

func TestDebugSessionFromTheRowMenuOpensInTheRepository(t *testing.T) {
	home := fractoryHome(t)
	m := buildModel(t)
	repo := filepath.Join(t.TempDir(), "books")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := m.services.store.AddAutomation(store.Automation{ID: "recon", Name: "reconcile", Config: agentAutomationDoc("recon", "reconcile", repo, ""), Enabled: true}, time.Now()); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 23, 7, 0, 0, 0, time.UTC)
	logPath := filepath.Join(home, "recon.log")
	writeFiringRecord(t, home, "recon-1", "recon", "done", "failed", at, at, at.Add(time.Minute), logPath)
	checkAutomations(t, m)
	showFirings(t, m, "recon")
	firing := uirail.Selection{Kind: uirail.FiringRow, Automation: "recon", Firing: "recon-1"}
	y0, _ := m.bodyYRange()
	m.handleMouse(tea.MouseMsg{X: 8, Y: y0 + railLineOf(t, m, firing), Action: tea.MouseActionPress, Button: tea.MouseButtonRight})
	if !m.rail.MenuOpen() {
		t.Fatal("a right click on the firing opened no menu")
	}
	entry := menuEntry(t, m, "Debug session")
	m, cmd := leftClickCmd(m, m.layout.displayedRail.MenuRect.Left+2, m.layout.displayedRail.MenuRect.Top+1+entry)
	m.applyCmd(t, cmd)
	m.drainEffects(t)

	groups, err := m.services.store.Groups()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(groups, func(g store.Group) bool { return g.Name == "books" && g.Path == repo }) {
		t.Fatalf("groups = %+v; want books on %s", groups, repo)
	}
	sessions, err := m.services.store.ListSessions(false)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions = %+v; want the Debug Session", sessions)
	}
	sess := sessions[0]
	if sess.Group != "books" || sess.Cwd != repo || sess.Name != "debug-reconcile-20261123-0700" {
		t.Fatalf("session = %+v", sess)
	}
	for _, want := range []string{`"reconcile"`, "outcome was failed (status done)", `"` + logPath + `"`, `"/wt/recon-1"`} {
		if !strings.Contains(sess.LaunchPrompt, want) {
			t.Fatalf("launch prompt lacks %q:\n%s", want, sess.LaunchPrompt)
		}
	}
}
