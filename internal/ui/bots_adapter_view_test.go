package ui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/status"
	"github.com/YoanWai/agent-manager/internal/store"
	uibots "github.com/YoanWai/agent-manager/internal/ui/bots"
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

var botProfileFixture = store.BotProfile{
	ID: "op", Name: "operator", Emoji: "🛠", Color: 1, Tool: "claude", Dir: "/work/fractory",
	Specialization: "fractory-operator", MemoryProject: "fractory-ops",
}

// listBots hands the model a poll that read the Profiles and sessions given.
func listBots(t *testing.T, m *Model, profiles []store.BotProfile, sessions []store.Session) {
	t.Helper()
	updated, _ := m.Update(refreshMsg{sessions: sessions, botProfiles: profiles, listedAt: time.Now().Add(time.Second)})
	*m = *updated.(*Model)
}

func botsDetail(t *testing.T, m *Model, selection uirail.Selection) string {
	t.Helper()
	if !m.rail.Focus(selection) {
		t.Fatalf("no row %+v in %+v", selection, m.rail.Rows())
	}
	lines, ok := m.botsContentLines(100)
	if !ok {
		t.Fatalf("no Bots detail for %+v", selection)
	}
	var text []string
	for _, line := range lines {
		text = append(text, ansi.Strip(line.Text))
	}
	return strings.Join(text, "\n")
}

func TestAProfileDetailShowsItsConfigurationAndLiveBots(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	listBots(t, m, []store.BotProfile{botProfileFixture}, []store.Session{
		{ID: "b1", Name: "watch runs", Tool: "claude", Cwd: "/work/fractory", Status: status.Working, BotProfile: "op"},
		{ID: "b2", Name: "watch host", Tool: "claude", Cwd: "/work/fractory", Status: status.Waiting, BotProfile: "op"},
		{ID: "plain", Name: "plain", Tool: "claude", Cwd: "/tmp", Status: status.Idle},
	})
	text := botsDetail(t, m, uirail.Selection{Kind: uirail.BotProfileRow, Profile: "op"})
	for _, want := range []string{"Profile operator", "Tool claude", "Model the CLI's own default", "Directory /work/fractory",
		"Specialization fractory-operator", "Memory fractory-ops", "Live Bots", "watch runs working", "watch host waiting"} {
		if !strings.Contains(text, want) {
			t.Fatalf("Profile detail lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "plain") {
		t.Fatalf("a plain session is listed as a Bot:\n%s", text)
	}
	if legend := m.rowLegend(); legend.Title != "Bot Profile" || !slices.ContainsFunc(legend.Pairs, func(p [2]string) bool { return p[1] == "show Bots" }) {
		t.Fatalf("Profile legend = %+v", legend)
	}

	// A Bot is the session it is: its detail is the session's, its legend a
	// session's.
	m.rail.Focus(uirail.Selection{Kind: uirail.BotProfileRow, Profile: "op"})
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.focusSession("b2") {
		t.Fatalf("an unfolded Profile does not list its Bot: %+v", m.rail.Rows())
	}
	if _, ok := m.botsContentLines(100); ok {
		t.Fatal("a Bot fills the detail column as a Profile")
	}
	if sess, ok := m.selected(); !ok || sess.ID != "b2" {
		t.Fatalf("selecting a Bot selects %+v", sess)
	}
}

func TestAnEmptyBotsAreaSaysHowToCreateAProfile(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	listBots(t, m, nil, []store.Session{{ID: "plain", Name: "plain", Tool: "claude", Cwd: "/tmp", Status: status.Idle}})
	text := botsDetail(t, m, uirail.Selection{Kind: uirail.BotsRow})
	if !strings.Contains(text, "No Bot Profiles yet") || !strings.Contains(text, uibots.CreateHint) {
		t.Fatalf("empty Bots detail:\n%s", text)
	}
	if view := ansi.Strip(preparedView(m)); !strings.Contains(view, "Bots (0)") {
		t.Fatalf("the rail lacks the empty Bots area:\n%s", view)
	}
}
