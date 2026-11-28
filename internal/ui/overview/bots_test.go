package overview

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/YoanWai/agent-manager/internal/ui/presentation"
)

func TestDrawBots(t *testing.T) {
	cases := []struct {
		name     string
		profiles []BotProfile
		want     []string
		absent   []string
	}{
		{"several live Bots in different states", []BotProfile{{Name: "operator", Emoji: "🛠", Tool: "claude", Bots: []Bot{
			{Name: "watch runs", Status: "working"}, {Name: "watch host", Status: "waiting"}, {Name: "old", Status: "errored"}}}},
			[]string{"Profiles 1", "Live Bots 3", "🛠 operator", "claude", "◐ watch runs working", "◆ watch host waiting", "! old errored"}, []string{"no live Bots"}},
		{"a Profile with none", []BotProfile{{Name: "idle-one", Tool: "codex"}}, []string{"Live Bots 0", "idle-one", "no live Bots"}, nil},
		{"no Profiles", nil, []string{"Profiles 0", "Live Bots 0"}, []string{"no live Bots"}},
	}
	for _, tc := range cases {
		for _, width := range []int{30, 100} {
			lines := DrawBots(tc.profiles, width, presentation.GlyphModeAdvanced, testPalette)
			text := strings.Join(lines, "\n")
			plainText := strings.Join(strings.Fields(ansi.Strip(text)), " ")
			for _, want := range tc.want {
				if width == 100 && !strings.Contains(plainText, want) {
					t.Errorf("%s: lacks %q:\n%s", tc.name, want, plainText)
				}
			}
			for _, absent := range tc.absent {
				if strings.Contains(plainText, absent) {
					t.Errorf("%s: has %q", tc.name, absent)
				}
			}
			for _, line := range lines {
				if w := ansi.StringWidth(line); w > width {
					t.Errorf("%s width %d: line is %d cells", tc.name, width, w)
				}
			}
		}
	}
}

func TestDrawBotsDimsAProfileWithNoLiveBotAndColoursTheRest(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
	lines := DrawBots([]BotProfile{{Name: "busy", Color: 1, Bots: []Bot{{Name: "b", Status: "working"}}}, {Name: "quiet", Color: 1}}, 100, presentation.GlyphModeAdvanced, testPalette)
	var busy, quiet string
	for _, line := range lines {
		switch {
		case strings.Contains(line, "busy"):
			busy = line
		case strings.Contains(line, "quiet"):
			quiet = line
		}
	}
	if !strings.Contains(busy, "38;2;0;255;0") {
		t.Errorf("a live Profile is not in its swatch: %q", busy)
	}
	if strings.Contains(quiet, "38;2;0;255;0") {
		t.Errorf("a Profile with no live Bot keeps its colour: %q", quiet)
	}
}

func TestDrawBotsCompatibleDrawsNoBraille(t *testing.T) {
	lines := DrawBots([]BotProfile{{Name: "p", Bots: []Bot{{Name: "b", Status: "working"}, {Name: "c", Status: "waiting"}}}}, 60, presentation.GlyphModeBasic, testPalette)
	for _, r := range strings.Join(lines, "") {
		if r >= 0x2800 && r <= 0x28FF {
			t.Fatal("drew braille")
		}
	}
	if !strings.Contains(strings.Join(strings.Fields(ansi.Strip(strings.Join(lines, "\n"))), " "), "* b working") {
		t.Fatalf("no ASCII dot: %q", lines)
	}
}
