package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/store"
	"github.com/YoanWai/agent-manager/internal/tmux"
	"github.com/YoanWai/agent-manager/internal/ui/bots"
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	"github.com/YoanWai/agent-manager/internal/ui/runs"
	uilaunch "github.com/YoanWai/agent-manager/internal/ui/spawn/launch"
	tea "github.com/charmbracelet/bubbletea"
)

func testBotProfile(dir string) bots.Profile {
	return bots.Profile{
		ID: "p1", Name: "Operator", Tool: "claude", Model: "opus", Dir: dir,
		Flags: "-u", Specialization: "fractory-operator", MemoryProject: "fractory-ops",
	}
}

func botPaneRuns(t *testing.T, m *Model, id string) bool {
	t.Helper()
	exists, err := m.services.tmux.SessionExists(id)
	if err != nil {
		t.Fatal(err)
	}
	return exists
}

func liveBots(t *testing.T, m *Model, profile string) []store.Session {
	t.Helper()
	list, err := m.services.store.ListSessions(false)
	if err != nil {
		t.Fatal(err)
	}
	var out []store.Session
	for _, sess := range list {
		if sess.BotProfile == profile {
			out = append(out, sess)
		}
	}
	return out
}

func TestSpawnBotOpensOneSessionOnTheProfile(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	dir := t.TempDir()
	if err := m.services.store.CreateGroup("ops", dir); err != nil {
		t.Fatal(err)
	}
	m.applyCmd(t, m.refreshCmd())
	m.applyCmd(t, m.spawnBot(testBotProfile(dir)))
	m.drainEffects(t)
	spawned := liveBots(t, m, "p1")
	if len(spawned) != 1 {
		t.Fatalf("Bots of p1 = %+v; want one", spawned)
	}
	bot := spawned[0]
	if bot.Tool != "claude" || bot.Cwd != dir || bot.Group != "ops" || bot.Name != "operator-1" {
		t.Fatalf("Bot = %+v; want claude in %s under ops named operator-1", bot, dir)
	}
	if bot.Choice.Model != "opus" || bot.Choice.Flags != "-u" {
		t.Fatalf("Bot choice = %+v; want the Profile's model and flags", bot.Choice)
	}
	if bot.LaunchPrompt != "" || len(bot.PendingInputs) != 0 {
		t.Fatalf("first prompt = %q %q; want an empty composer", bot.LaunchPrompt, bot.PendingInputs)
	}
	if marker, err := os.ReadFile(filepath.Join(dir, bots.MemoryMarker)); err != nil || !strings.Contains(string(marker), `"fractory-ops"`) {
		t.Fatalf("memory marker = %q, %v; want the Profile's project", marker, err)
	}
	if !botPaneRuns(t, m, bot.ID) {
		t.Fatalf("no pane runs the Bot %s", bot.ID)
	}
}

func TestSpawnBotWithNoDirectoryStartsInItsOwnUnderTheBotsRoot(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	m.env.botsDir = filepath.Join(t.TempDir(), "Work", "Bots")
	profile := testBotProfile("")
	profile.Specialization = ""
	m.applyCmd(t, m.spawnBot(profile))
	m.drainEffects(t)
	spawned := liveBots(t, m, "p1")
	want := filepath.Join(m.env.botsDir, "operator")
	if len(spawned) != 1 || spawned[0].Cwd != want {
		t.Fatalf("Bots of p1 = %+v; want one in %s", spawned, want)
	}
	marker, err := os.ReadFile(filepath.Join(want, bots.MemoryMarker))
	if err != nil || !strings.Contains(string(marker), `project = "fractory-ops"`) || !strings.Contains(string(marker), "workspace = ") {
		t.Fatalf("memory marker = %q, %v; want the workspace and the Profile's project", marker, err)
	}
}

func TestSpawnBotInARepositoryWritesNoMarker(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	m.env.botsDir = filepath.Join(t.TempDir(), "Work", "Bots")
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	m.applyCmd(t, m.spawnBot(testBotProfile(repo)))
	m.drainEffects(t)
	if spawned := liveBots(t, m, "p1"); len(spawned) != 1 || spawned[0].Cwd != repo {
		t.Fatalf("Bots of p1 = %+v; want one in %s", spawned, repo)
	}
	for _, dir := range []string{repo, m.env.botsDir} {
		err := filepath.WalkDir(dir, func(path string, _ os.DirEntry, err error) error {
			if err == nil && filepath.Base(path) == bots.MemoryMarker {
				t.Errorf("a marker was written at %s", path)
			}
			return err
		})
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
}

// specializingModel is a Model whose claude appends a file to its system
// prompt, with the owner's home in a temp directory and a Run registry that
// names two repository checkouts. repo is the second of them, so a skill
// found there was looked for past one that holds none.
func specializingModel(t *testing.T) (m *Model, repo, home string) {
	t.Helper()
	m = buildModel(t)
	tool := m.services.cfg.Tools["claude"]
	tool.AppendSystemPromptFlag = "--append-system-prompt-file"
	m.services.cfg.Tools["claude"] = tool
	parent := t.TempDir()
	other, repo := filepath.Join(parent, "a-other"), filepath.Join(parent, "b-fractory")
	m.workspace.runs.list = []runs.Record{{ID: "r1", RepoPath: repo}, {ID: "r2", RepoPath: other}, {ID: "r3", RepoPath: repo}}
	m.env.homeDir = t.TempDir()
	return m, repo, m.env.homeDir
}

func writeSkill(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, name, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\nname: "+name+"\n---\nWatch the Runs.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A skill committed in a repository checkout the Run registry names wins over
// the owner's global skill of the same name.
func TestSpawnBotPassesItsSpecializationAsASystemPromptFile(t *testing.T) {
	t.Parallel()
	m, repo, home := specializingModel(t)
	writeSkill(t, filepath.Join(home, ".claude", "skills"), "fractory-operator")
	want := writeSkill(t, filepath.Join(repo, ".agents", "skills"), "fractory-operator")
	m.applyCmd(t, m.spawnBot(testBotProfile(t.TempDir())))
	m.drainEffects(t)
	spawned := liveBots(t, m, "p1")
	if len(spawned) != 1 {
		t.Fatalf("Bots of p1 = %+v; want one", spawned)
	}
	flags := "-u --append-system-prompt-file " + tmux.ShellQuote(want)
	if got := spawned[0].Choice.Flags; got != flags {
		t.Fatalf("launch flags = %q; want %q, the repository skill ahead of the global one", got, flags)
	}
	if !botPaneRuns(t, m, spawned[0].ID) {
		t.Fatalf("no pane runs the Bot %s", spawned[0].ID)
	}
}

func TestSpawnBotFallsBackToTheGlobalSkill(t *testing.T) {
	t.Parallel()
	m, _, home := specializingModel(t)
	want := writeSkill(t, filepath.Join(home, ".claude", "skills"), "fractory-operator")
	m.applyCmd(t, m.spawnBot(testBotProfile(t.TempDir())))
	m.drainEffects(t)
	spawned := liveBots(t, m, "p1")
	if len(spawned) != 1 || !strings.HasSuffix(spawned[0].Choice.Flags, tmux.ShellQuote(want)) {
		t.Fatalf("Bots of p1 = %+v; want one launched on %s", spawned, want)
	}
}

func TestSpawnBotRefusesASpecializationWithNoSkillFile(t *testing.T) {
	t.Parallel()
	m, _, _ := specializingModel(t)
	m.applyCmd(t, m.spawnBot(testBotProfile(t.TempDir())))
	m.drainEffects(t)
	if spawned := liveBots(t, m, "p1"); len(spawned) != 0 {
		t.Fatalf("a Specialization with no skill started %+v", spawned)
	}
	for _, want := range []string{"Operator", "fractory-operator", "SKILL.md"} {
		if !strings.Contains(m.errBar.text, want) {
			t.Fatalf("status = %q; want %q named", m.errBar.text, want)
		}
	}
}

func TestSpawnBotAddsNoFlagWithoutASpecializationOrTheCapability(t *testing.T) {
	t.Parallel()
	m, repo, _ := specializingModel(t)
	writeSkill(t, filepath.Join(repo, ".agents", "skills"), "fractory-operator")
	plain := testBotProfile(t.TempDir())
	plain.Specialization = ""
	m.applyCmd(t, m.spawnBot(plain))
	m.drainEffects(t)
	incapable, _, _ := specializingModel(t)
	tool := incapable.services.cfg.Tools["claude"]
	tool.AppendSystemPromptFlag = ""
	incapable.services.cfg.Tools["claude"] = tool
	incapable.applyCmd(t, incapable.spawnBot(testBotProfile(t.TempDir())))
	incapable.drainEffects(t)
	for _, spawned := range [][]store.Session{liveBots(t, m, "p1"), liveBots(t, incapable, "p1")} {
		if len(spawned) != 1 || spawned[0].Choice.Flags != "-u" {
			t.Fatalf("Bots = %+v; want one on the Profile's own flags", spawned)
		}
	}
}

func TestTwoBotsFromOneProfileRunSideBySide(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	dir := t.TempDir()
	profile := testBotProfile(dir)
	m.applyCmd(t, m.spawnBot(profile))
	m.applyCmd(t, m.spawnBot(profile))
	m.drainEffects(t)
	spawned := liveBots(t, m, "p1")
	if len(spawned) != 2 {
		t.Fatalf("Bots of p1 = %+v; want two", spawned)
	}
	if spawned[0].Name == spawned[1].Name || spawned[0].Group != spawned[1].Group {
		t.Fatalf("Bots %q in %q and %q in %q; want two names in one group", spawned[0].Name, spawned[0].Group, spawned[1].Name, spawned[1].Group)
	}
	for _, bot := range spawned {
		if !botPaneRuns(t, m, bot.ID) {
			t.Fatalf("no pane runs the Bot %s", bot.Name)
		}
	}
}

func TestSpawnBotRefusesAProfileOnAnUnknownCLI(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	profile := testBotProfile(t.TempDir())
	profile.Tool = "gone"
	m.applyCmd(t, m.spawnBot(profile))
	for _, request := range queuedMainEffects(m) {
		if spawn, ok := request.(uilaunch.Request); ok {
			t.Fatalf("an unknown CLI queued %+v", spawn)
		}
	}
	if !strings.Contains(m.errBar.text, "gone") {
		t.Fatalf("status = %q; want the CLI named", m.errBar.text)
	}
}

// A Bot is a session: rename, focus and archive take it like any other and
// keep the Profile it records.
func TestABotIsAnOrdinarySession(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	m.applyCmd(t, m.spawnBot(testBotProfile(t.TempDir())))
	m.drainEffects(t)
	m.selectSessionRow(t, "operator-1")
	m.openRename()
	openRename(m).SetName("watching-runs")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m.applyCmd(t, cmd)
	renamed := liveBots(t, m, "p1")
	if len(renamed) != 1 || renamed[0].Name != "watching-runs" {
		t.Fatalf("after rename Bots = %+v", renamed)
	}
	m.selectSessionRow(t, "watching-runs")
	m.applyCmd(t, m.enterFocus(renamed[0]))
	if !m.focused() {
		t.Fatal("focus mode did not open on the Bot")
	}
	m.applyCmd(t, m.leaveFocus())
	m.selectSessionRow(t, "watching-runs")
	m.archiveSelected()
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m.applyCmd(t, cmd)
	m.drainEffects(t)
	if live := liveBots(t, m, "p1"); len(live) != 0 {
		t.Fatalf("archived Bot still live: %+v", live)
	}
	archived, err := m.services.store.Get(renamed[0].ID)
	if err != nil || !archived.Archived || archived.BotProfile != "p1" {
		t.Fatalf("archived Bot = %+v, %v; want archived, still of p1", archived, err)
	}
}

// B on a stored Profile's row spawns from what the store holds, and the poll
// after it lists each Bot under that Profile.
func TestTheSpawnKeyOnAProfileRowListsTheBotsUnderIt(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	dir := t.TempDir()
	if err := m.services.store.AddBotProfile(store.BotProfile{ID: "p1", Name: "Operator", Tool: "claude", Dir: dir, Specialization: "fractory-operator"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	m.applyCmd(t, m.refreshCmd())
	profile := uirail.Selection{Kind: uirail.BotProfileRow, Profile: "p1"}
	for range 2 {
		if !m.rail.Focus(profile) {
			t.Fatalf("no Profile row in %+v", m.rail.Rows())
		}
		_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("B")})
		m.applyCmd(t, cmd)
		m.drainEffects(t)
	}
	m.applyCmd(t, m.refreshCmd())
	spawned := liveBots(t, m, "p1")
	if len(spawned) != 2 {
		t.Fatalf("Bots of p1 = %+v; want two", spawned)
	}
	m.rail.Focus(profile)
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	for _, bot := range spawned {
		if !m.focusSession(bot.ID) {
			t.Fatalf("the unfolded Profile does not list %s: %+v", bot.Name, m.rail.Rows())
		}
	}
	m.rail.Focus(profile)
	if legend, _ := m.botsLegend(); !slices.ContainsFunc(legend.Pairs, func(p [2]string) bool { return p[1] == "spawn a Bot" }) {
		t.Fatalf("Profile footer = %+v; want the spawn key", legend)
	}
}
