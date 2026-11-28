package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/store"
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// toastText is what the card floating over the frame's top right reads, its
// rows joined with no space between them, or "" when no card is up.
func toastText(frame string) string {
	rows := strings.Split(frame, "\n")
	for i, row := range rows {
		plain := ansi.Strip(row)
		corner := strings.Index(plain, "╭")
		if corner < 0 {
			continue
		}
		left := ansi.StringWidth(plain[:corner])
		width := ansi.StringWidth(strings.TrimRight(plain[corner:], " "))
		var text strings.Builder
		for _, below := range rows[i+1:] {
			cell := ansi.Strip(ansi.Cut(below, left, left+width))
			if strings.HasPrefix(cell, "╰") || !strings.HasPrefix(cell, "│") {
				break
			}
			text.WriteString(strings.TrimSpace(strings.Trim(cell, "│")))
		}
		return text.String()
	}
	return ""
}

// noSpaces drops every space, so words a card wrapped onto two rows compare
// equal to the text they came from.
func noSpaces(s string) string { return strings.ReplaceAll(s, " ", "") }

// Each way B on a Profile can refuse to start a Bot puts its reason on the
// frame and keeps it there through the passes the live program polls while
// the lane is busy: the poke a finished effect asks for, the surfaces read
// and the ticker.
func TestARefusedBotSpawnShowsItsReasonOnTheFrame(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		profile store.BotProfile
		want    func(repo, home string) []string
	}{
		{
			name:    "a Specialization with no skill file",
			profile: store.BotProfile{ID: "p1", Name: "broken", Tool: "claude", Specialization: "does-not-exist"},
			want: func(repo, home string) []string {
				return []string{"broken", `"does-not-exist"`, ", " +
					filepath.Join(repo, ".agents", "skills", "does-not-exist", "SKILL.md") + ", " +
					filepath.Join(home, ".claude", "skills", "does-not-exist", "SKILL.md")}
			},
		},
		{
			name:    "an unknown CLI",
			profile: store.BotProfile{ID: "p1", Name: "stale", Tool: "gone"},
			want:    func(string, string) []string { return []string{"stale", "gone", "not an agent CLI"} },
		},
		{
			name:    "a tool that is not an agent CLI",
			profile: store.BotProfile{ID: "p1", Name: "shelly", Tool: "terminal"},
			want:    func(string, string) []string { return []string{"shelly", "terminal", "not an agent CLI"} },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m, repo, home := specializingModel(t)
			tc.profile.Dir = t.TempDir()
			if err := m.services.store.AddBotProfile(tc.profile, time.Now()); err != nil {
				t.Fatal(err)
			}
			m.applyCmd(t, m.refreshCmd())
			if !m.rail.Focus(uirail.Selection{Kind: uirail.BotProfileRow, Profile: "p1"}) {
				t.Fatalf("no Profile row in %+v", m.rail.Rows())
			}
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("B")})
			m.applyCmd(t, cmd)
			for range 4 {
				m.applyCmd(t, m.refreshCmd())
			}
			if spawned := liveBots(t, m, "p1"); len(spawned) != 0 {
				t.Fatalf("a refused Profile started %+v", spawned)
			}
			toast := toastText(m.View())
			for _, want := range tc.want(repo, home) {
				if !strings.Contains(noSpaces(toast), noSpaces(want)) {
					t.Fatalf("the frame reads %q; want %q on it:\n%s", toast, want, ansi.Strip(m.View()))
				}
			}
		})
	}
}
