package ui

import (
	"strconv"

	"github.com/YoanWai/agent-manager/internal/keybind"
	"github.com/YoanWai/agent-manager/internal/store"
	uibots "github.com/YoanWai/agent-manager/internal/ui/bots"
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	uitheme "github.com/YoanWai/agent-manager/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func railBotProfiles(profiles []store.BotProfile) []uirail.BotProfile {
	out := make([]uirail.BotProfile, len(profiles))
	for i, p := range profiles {
		out[i] = uirail.BotProfile{ID: p.ID, Name: p.Name, Emoji: p.Emoji, Color: p.Color, Tool: p.Tool}
	}
	return out
}

func botProfileOf(p store.BotProfile) uibots.Profile {
	return uibots.Profile{
		ID: p.ID, Name: p.Name, Emoji: p.Emoji, Color: p.Color, Tool: p.Tool, Model: p.Model, Dir: p.Dir,
		Flags: p.Flags, Specialization: p.Specialization, MemoryProject: p.MemoryProject,
	}
}

func isBotsRow(kind uirail.RowKind) bool {
	return kind == uirail.BotsRow || kind == uirail.BotProfileRow
}

// selectedBotProfile is the Profile the cursor is on.
func (m *Model) selectedBotProfile() (store.BotProfile, bool) {
	selection, ok := m.rail.Selected()
	if !ok || selection.Kind != uirail.BotProfileRow {
		return store.BotProfile{}, false
	}
	for _, profile := range m.workspace.botProfiles {
		if profile.ID == selection.Profile {
			return profile, true
		}
	}
	return store.BotProfile{}, false
}

// liveBots are the running sessions started from the Profile id.
func (m *Model) liveBots(id string) []store.Session {
	var bots []store.Session
	for _, sess := range m.workspace.sessions {
		if sess.BotProfile == id && !sess.Archived {
			bots = append(bots, sess)
		}
	}
	return bots
}

// botsIntent runs a rail intent aimed at the Bots header or a Profile and
// reports whether it took it. The keys that reach the whole workspace pass
// through, and a Profile spawns a Bot; a live Bot is a session row and never
// arrives here.
func (m *Model) botsIntent(intent uirail.Intent) (tea.Cmd, bool) {
	if !isBotsRow(intent.Target.Kind) {
		return nil, false
	}
	switch intent.Kind {
	case uirail.Quit, uirail.OpenSettings, uirail.Resize, uirail.OpenMessages, uirail.OpenHelp, uirail.Prompt, uirail.NewSession, uirail.NewGroup, uirail.NewConnection:
		return nil, false
	case uirail.SpawnBot:
		if profile, ok := m.selectedBotProfile(); ok {
			return m.spawnBot(botProfileOf(profile)), true
		}
		return nil, true
	}
	m.reportErr("a Bot Profile is made and edited from a shell: agent-manager bots")
	return nil, true
}

func (m *Model) botsLegend() (legendSection, bool) {
	selection, ok := m.rail.Selected()
	if !ok || !isBotsRow(selection.Kind) {
		return legendSection{}, false
	}
	action, mark := "fold", m.glyphs().GroupOpenGlyph
	if selection.Kind == uirail.BotProfileRow {
		action = "hide Bots"
	}
	if m.rail.SelectionCollapsed(selection) {
		action, mark = "unfold", m.glyphs().GroupClosedGlyph
		if selection.Kind == uirail.BotProfileRow {
			action = "show Bots"
		}
	}
	pairs := [][2]string{{m.listGlyph(keybind.Open), action}}
	if !m.prefs.mouseDisabled {
		pairs = append(pairs, [2]string{"click " + mark, action})
	}
	if selection.Kind == uirail.BotProfileRow {
		pairs = append(pairs, [2]string{m.listGlyph(keybind.SpawnBot), "spawn a Bot"})
	}
	title := "Bots"
	if selection.Kind == uirail.BotProfileRow {
		title = "Bot Profile"
	}
	return legendSection{Title: title, Pairs: pairs}, true
}

// botsContentLines fills the content column for the Bots header and a
// Profile: what the area holds or how to start it, or the Profile's
// configuration and its live Bots.
func (m *Model) botsContentLines(width int) ([]contentLine, bool) {
	selection, ok := m.rail.Selected()
	if !ok || !isBotsRow(selection.Kind) {
		return nil, false
	}
	gutter := "  "
	inner := max(width-len(gutter)*2, 10)
	var lines []contentLine
	wrap := func(text string) {
		for _, line := range uitheme.SplitLines(ansi.Wrap(text, inner, "")) {
			lines = append(lines, contentLine{Text: gutter + line})
		}
	}
	add := func(label, value string) {
		wrap(uitheme.LabelStyle().Render(label) + " " + uitheme.ValueStyle().Render(value))
	}
	if selection.Kind == uirail.BotsRow {
		if len(m.workspace.botProfiles) == 0 {
			wrap(uitheme.LabelStyle().Render("No Bot Profiles yet. A Profile is a specialized session set up once; create one from a shell:"))
			lines = append(lines, contentLine{})
			wrap(uitheme.ValueStyle().Render(uibots.CreateHint))
			return lines, true
		}
		live := 0
		for _, profile := range m.workspace.botProfiles {
			live += len(m.liveBots(profile.ID))
		}
		add("Profiles", strconv.Itoa(len(m.workspace.botProfiles)))
		add("Live Bots", strconv.Itoa(live))
		lines = append(lines, contentLine{})
		wrap(uitheme.LabelStyle().Render("A Bot is an ordinary session started from a Profile; agent-manager bots lists and edits them"))
		return lines, true
	}
	stored, ok := m.selectedBotProfile()
	if !ok {
		return lines, true
	}
	profile := botProfileOf(stored)
	swatches := m.railTheme().Swatches()
	color := swatches[((profile.Color%len(swatches))+len(swatches))%len(swatches)]
	for _, fact := range profile.Facts() {
		if fact[0] == "Profile" {
			wrap(uitheme.LabelStyle().Render(fact[0]) + " " + lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(fact[1]))
			continue
		}
		add(fact[0], fact[1])
	}
	lines = append(lines, contentLine{})
	bots := m.liveBots(profile.ID)
	if len(bots) == 0 {
		wrap(uitheme.LabelStyle().Render("No live Bots"))
		return lines, true
	}
	lines = append(lines, contentLine{Text: gutter + uitheme.LabelStyle().Render("Live Bots")})
	for _, bot := range bots {
		add(bot.Name, bot.Status)
	}
	return lines, true
}
