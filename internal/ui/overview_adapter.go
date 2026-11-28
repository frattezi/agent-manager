package ui

import (
	"strconv"
	"time"

	"github.com/YoanWai/agent-manager/internal/keybind"
	uioverview "github.com/YoanWai/agent-manager/internal/ui/overview"
	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	"github.com/YoanWai/agent-manager/internal/ui/runs"
	uitheme "github.com/YoanWai/agent-manager/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// overviewWheelStep is the lines one wheel notch scrolls the Overview.
const overviewWheelStep = 3

// overviewState is the Runs Overview's view state. It lives in memory: the
// window starts at all every run of the manager, and the scroll resets when
// the selection leaves the header.
type overviewState struct {
	window uioverview.Window
	scroll int
	// windowRow is the content row the window label was last painted on, -1
	// when it is scrolled out of the pane or the Overview is not showing.
	windowRow int
	// pageRows is the rows a page key scrolls.
	pageRows int
	// shown is the header whose Overview was last painted, so the scroll
	// starts at the top when the selection moves to the other one.
	shown uirail.RowKind
	cache overviewCache
}

type overviewCacheKey struct {
	window uioverview.Window
	width  int
	minute time.Time
	theme  string
	basic  bool
	key    string
	// automations is the header the drawing is for.
	automations bool
}

type overviewCache struct {
	valid   bool
	key     overviewCacheKey
	drawing uioverview.Drawing
}

func (m *Model) runsHeaderSelected() bool {
	selection, ok := m.rail.Selected()
	return ok && selection.Kind == uirail.RunsRow
}

// overviewHeaderSelected is true on the header of a section whose Overview
// takes the shared window and scroll.
func (m *Model) overviewHeaderSelected() bool {
	selection, ok := m.rail.Selected()
	return ok && (selection.Kind == uirail.RunsRow || selection.Kind == uirail.AutomationsRow)
}

func (m *Model) botsHeaderSelected() bool {
	selection, ok := m.rail.Selected()
	return ok && selection.Kind == uirail.BotsRow
}

// overviewScrollable is true on the header of any section with an Overview:
// the ones that take the window, and Bots, which is live and has none.
func (m *Model) overviewScrollable() bool {
	return m.overviewHeaderSelected() || m.botsHeaderSelected()
}

// botsOverviewLines is the Bots Overview, live state only; nil when no Profile
// is stored, which leaves the header's hint on how to create one.
func (m *Model) botsOverviewLines(width int) []string {
	if len(m.workspace.botProfiles) == 0 {
		return nil
	}
	profiles := make([]uioverview.BotProfile, len(m.workspace.botProfiles))
	for i, profile := range m.workspace.botProfiles {
		profiles[i] = uioverview.BotProfile{Name: profile.Name, Emoji: profile.Emoji, Color: profile.Color, Tool: profile.Tool}
		for _, bot := range m.liveBots(profile.ID) {
			profiles[i].Bots = append(profiles[i].Bots, uioverview.Bot{Name: bot.Name, Status: bot.Status})
		}
	}
	return uioverview.DrawBots(profiles, width, m.prefs.glyphMode.Effective(), overviewPalette(uitheme.Current()))
}

func overviewPalette(theme uitheme.Theme) uioverview.Palette {
	return uioverview.Palette{
		Swatches: theme.Swatches(), Working: theme.Working, Waiting: theme.Waiting, Finished: theme.Finished,
		Errored: theme.Errored, Idle: theme.Idle, Bright: theme.Bright, Text: theme.Text, Dim: theme.Dim,
	}
}

// overviewContentLines fills the content column for a section header with an
// Overview: for Runs the held line when the queue is Held, then the Overview,
// scrolled to fit the pane.
func (m *Model) overviewContentLines(width, height int) ([]contentLine, bool) {
	state := &m.workspace.overview
	windowed := m.overviewHeaderSelected()
	var botLines []string
	if m.botsHeaderSelected() {
		botLines = m.botsOverviewLines(width - 4)
	}
	if !windowed && botLines == nil {
		state.scroll, state.windowRow = 0, -1
		return nil, false
	}
	if state.shown != m.selectedOverview() {
		state.shown, state.scroll = m.selectedOverview(), 0
	}
	const gutter = "  "
	now := time.Now()
	var lines []contentLine
	for _, line := range botLines {
		lines = append(lines, contentLine{Text: gutter + line})
	}
	if held := runs.HeldLine(m.workspace.runs.host, now); held != "" && m.runsHeaderSelected() {
		for _, line := range uitheme.SplitLines(ansi.Wrap(held, max(width-len(gutter)*2, 10), "")) {
			lines = append(lines, contentLine{Text: gutter + uitheme.WarnStyle().Render(line)})
		}
		lines = append(lines, contentLine{})
	}
	windowRow := -1
	if windowed {
		drawing := m.overviewDrawing(width-len(gutter)*2, now)
		for _, line := range drawing.Lines {
			lines = append(lines, contentLine{Text: gutter + line})
		}
		windowRow = len(lines) - len(drawing.Lines) + drawing.WindowRow
	}

	state.pageRows = max(height-2, 1)
	if len(lines) > height {
		state.scroll = min(max(state.scroll, 0), len(lines)-(height-1))
		above, shown := state.scroll, lines[state.scroll:]
		shown = shown[:height-1]
		below := len(lines) - above - len(shown)
		hint := "↑ " + strconv.Itoa(above) + " above · ↓ " + strconv.Itoa(below) + " below"
		lines = append(shown, contentLine{Text: gutter + uitheme.MutedStyle().Render(hint)})
		windowRow -= above
	} else {
		state.scroll = 0
	}
	state.windowRow = windowRow
	if windowRow < 0 || windowRow >= len(lines) {
		state.windowRow = -1
	}
	return lines, true
}

func (m *Model) selectedOverview() uirail.RowKind {
	selection, _ := m.rail.Selected()
	return selection.Kind
}

func (m *Model) overviewDrawing(width int, now time.Time) uioverview.Drawing {
	state := &m.workspace.overview
	theme := uitheme.Current()
	basic := m.prefs.glyphMode.Effective() != presentation.GlyphModeAdvanced
	key := overviewCacheKey{window: state.window, width: width, minute: now.Truncate(time.Minute), theme: theme.Name, basic: basic, key: m.listGlyph(keybind.OverviewWindow), automations: !m.runsHeaderSelected()}
	if state.cache.valid && state.cache.key == key {
		return state.cache.drawing
	}
	palette := overviewPalette(theme)
	var drawing uioverview.Drawing
	if key.automations {
		computed := uioverview.ComputeAutomations(m.workspace.automations.entries, state.window, now)
		drawing = uioverview.DrawAutomations(computed, width, key.key, m.prefs.glyphMode.Effective(), palette)
	} else {
		computed := uioverview.Compute(m.workspace.runs.list, state.window, now)
		drawing = uioverview.Draw(computed, width, key.key, m.prefs.glyphMode.Effective(), palette)
	}
	state.cache = overviewCache{valid: true, key: key, drawing: drawing}
	return drawing
}

// overviewKey runs the Overview's keys while a header with an Overview is
// selected and the list is not taking text: the window step on the headers
// that take the window, and the page scroll on all of them.
func (m *Model) overviewKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if !m.overviewScrollable() || m.rail.InputMode() != uirail.BrowseMode {
		return nil, false
	}
	action, _ := m.services.listKeys.ActionFor(keybind.Normalize(msg.String()))
	switch action {
	case keybind.OverviewWindow:
		if !m.overviewHeaderSelected() {
			return nil, false
		}
		m.stepOverviewWindow()
	case keybind.OverviewUp:
		m.scrollOverview(-m.workspace.overview.pageRows)
	case keybind.OverviewDown:
		m.scrollOverview(m.workspace.overview.pageRows)
	default:
		return nil, false
	}
	return nil, true
}

func (m *Model) stepOverviewWindow() {
	m.workspace.overview.window = m.workspace.overview.window.Next()
	m.workspace.overview.scroll = 0
}

// scrollOverview moves the Overview by delta rows; the next paint clamps it.
func (m *Model) scrollOverview(delta int) {
	m.workspace.overview.scroll = max(m.workspace.overview.scroll+delta, 0)
}

// overviewWheel scrolls the Overview for a wheel over the content column.
func (m *Model) overviewWheel(msg tea.MouseMsg) bool {
	if !m.overviewScrollable() || !m.onContentColumn(msg.X, msg.Y) {
		return false
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		m.scrollOverview(-overviewWheelStep)
	case tea.MouseButtonWheelDown:
		m.scrollOverview(overviewWheelStep)
	default:
		return false
	}
	return true
}

// overviewClick steps the window when a press lands on its label.
func (m *Model) overviewClick(msg tea.MouseMsg) bool {
	if !m.overviewHeaderSelected() || m.workspace.overview.windowRow < 0 {
		return false
	}
	y0, _ := m.bodyYRange()
	if msg.Y-y0 != m.workspace.overview.windowRow {
		return false
	}
	m.stepOverviewWindow()
	return true
}
