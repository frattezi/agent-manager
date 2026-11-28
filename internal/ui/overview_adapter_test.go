package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/keybind"
	"github.com/YoanWai/agent-manager/internal/status"
	"github.com/YoanWai/agent-manager/internal/store"
	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	"github.com/YoanWai/agent-manager/internal/ui/runs"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func overviewRuns(projects int) []runs.Record {
	now := time.Now()
	var list []runs.Record
	for i := range projects {
		name := fmt.Sprintf("proj%d", i)
		list = append(list, runs.Record{
			Version: 1, ID: fmt.Sprintf("o%d", i), TicketRef: fmt.Sprintf("o/%s#1", name), Status: runs.StatusDone, RepoPath: "/src/" + name,
			Branch: "fractory/1-ship-" + name, LogPath: "/tmp/o.log", CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-time.Duration(i) * time.Minute),
			FinishedAt: now.Add(-time.Hour), Result: &runs.Result{Summary: "Shipped " + name + " end to end."}, Mode: runs.ModeWork,
		})
	}
	return list
}

func selectRunsHeader(t *testing.T, m *Model) {
	t.Helper()
	if !m.rail.Focus(uirail.Selection{Kind: uirail.RunsRow}) {
		t.Fatal("no Runs header")
	}
}

func overviewText(m *Model, width, height int) string {
	var out []string
	for _, line := range m.contentLines(width, height) {
		out = append(out, ansi.Strip(line.Text))
	}
	return strings.Join(out, "\n")
}

func TestRunsHeaderShowsTheOverviewAtNarrowAndWideWidths(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, overviewRuns(2))
	selectRunsHeader(t, m)
	for _, width := range []int{40, 110} {
		text := overviewText(m, width, 60)
		for _, want := range []string{"proj0", "proj1", "Window all", "Shipped proj0 end to end."} {
			if !strings.Contains(strings.Join(strings.Fields(strings.ReplaceAll(text, "│", " ")), " "), want) {
				t.Errorf("width %d lacks %q:\n%s", width, want, text)
			}
		}
	}
}

func TestOverviewWindowStepsByKeyAndByClick(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, overviewRuns(1))
	selectRunsHeader(t, m)
	overviewText(m, 100, 40)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("W")})
	if text := overviewText(m, 100, 40); !strings.Contains(text, "Window 24h") {
		t.Fatalf("after the key:\n%s", text)
	}
	y0, _ := m.bodyYRange()
	m.handleMouse(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: m.layout.width - 3, Y: y0 + m.workspace.overview.windowRow})
	if text := overviewText(m, 100, 40); !strings.Contains(text, "Window 7d") {
		t.Fatalf("after the click:\n%s", text)
	}
}

func TestTallOverviewScrollsByKeysAndWheel(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, overviewRuns(6))
	selectRunsHeader(t, m)
	y0, y1 := m.bodyYRange()
	h := y1 - y0
	first := overviewText(m, 100, h)
	if strings.Contains(first, "proj5") || !strings.Contains(first, "below") {
		t.Fatalf("the pane shows everything:\n%s", first)
	}
	for range 10 {
		m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
		overviewText(m, 100, h)
	}
	if last := overviewText(m, 100, h); !strings.Contains(last, "proj5") || !strings.Contains(last, "0 below") {
		t.Fatalf("paging down never reaches the last Project:\n%s", last)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	up := overviewText(m, 100, h)
	for range 60 {
		m.handleMouse(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp, X: m.layout.width - 3, Y: 5})
		up = overviewText(m, 100, h)
	}
	if !strings.Contains(up, "0 above") || !strings.Contains(up, "Window") {
		t.Fatalf("the wheel does not scroll back to the top:\n%s", up)
	}
}

func TestRunRowStillShowsItsOwnDetailAndScrollResets(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, overviewRuns(6))
	selectRunsHeader(t, m)
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	overviewText(m, 100, 20)
	selectRun(t, m, "o0")
	text := overviewText(m, 100, 20)
	if !strings.Contains(text, "Ticket") || strings.Contains(text, "Window") || m.workspace.overview.scroll != 0 {
		t.Fatalf("a Run row lost its detail or the scroll stayed:\n%s", text)
	}
}

func TestOverviewHeldLineStaysAtTheTop(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	host := &runs.HostState{Held: true, Reason: "Docker memory low", HeldSince: time.Now().Add(-time.Minute)}
	m.applyTestMsg(t, runsLoadedMsg{dir: "/registry", list: overviewRuns(1), host: host})
	selectRunsHeader(t, m)
	if text := overviewText(m, 100, 40); !strings.HasPrefix(strings.TrimSpace(text), "Held") {
		t.Fatalf("Held line is not first:\n%s", text)
	}
}

func TestOverviewDrawsNoBrailleUnderCompatibleGlyphs(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	list := overviewRuns(1)
	list = append(list, list[0])
	list[1].ID, list[1].TicketRef = "o9", "o/proj0#2"
	loadRuns(t, m, list)
	selectRunsHeader(t, m)
	m.prefs.glyphMode = presentation.GlyphModeBasic
	for _, r := range overviewText(m, 100, 60) {
		if r >= 0x2800 && r <= 0x28FF {
			t.Fatal("Compatible glyph mode drew braille")
		}
	}
}

func TestOverviewKeysAreNamedInTheFooterAndKeyMap(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, overviewRuns(1))
	selectRunsHeader(t, m)
	legend, _ := m.runLegend()
	var window, scroll bool
	for _, pair := range legend.Pairs {
		window = window || pair == [2]string{m.listGlyph(keybind.OverviewWindow), "window 24h"}
		scroll = scroll || pair[1] == "scroll"
	}
	if !window || !scroll {
		t.Fatalf("footer pairs = %+v", legend.Pairs)
	}
}

func TestEmptyOverviewSaysSoInOneLine(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	loadRuns(t, m, overviewRuns(1))
	selectRunsHeader(t, m)
	m.workspace.overview.window = m.workspace.overview.window.Next() // 24h
	m.workspace.overview.window = m.workspace.overview.window.Next() // 7d
	loadRuns(t, m, []runs.Record{func() runs.Record {
		r := overviewRuns(1)[0]
		r.UpdatedAt = time.Now().Add(-40 * 24 * time.Hour)
		return r
	}()})
	if text := overviewText(m, 100, 30); !strings.Contains(text, "No Runs in the last 7d") {
		t.Fatalf("empty window:\n%s", text)
	}
}

func botsOverviewFixture(t *testing.T, m *Model, profiles int) {
	t.Helper()
	var ps []store.BotProfile
	var sessions []store.Session
	for i := range profiles {
		id := fmt.Sprintf("p%d", i)
		ps = append(ps, store.BotProfile{ID: id, Name: fmt.Sprintf("profile%d", i), Emoji: "🛠", Color: i, Tool: "claude", Dir: "/w"})
		if i%2 == 0 {
			sessions = append(sessions, store.Session{ID: "b" + id, Name: "bot-" + id, Tool: "claude", Cwd: "/w", Status: status.Working, BotProfile: id})
		}
	}
	listBots(t, m, ps, sessions)
	if !m.rail.Focus(uirail.Selection{Kind: uirail.BotsRow}) {
		t.Fatal("no Bots header")
	}
}

func TestBotsHeaderShowsTheOverviewAtNarrowAndWideWidths(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	botsOverviewFixture(t, m, 2)
	for _, width := range []int{40, 110} {
		text := strings.Join(strings.Fields(overviewText(m, width, 60)), " ")
		for _, want := range []string{"Profiles 2", "Live Bots 1", "🛠 profile0", "bot-p0 working", "profile1", "no live Bots"} {
			if !strings.Contains(text, want) {
				t.Errorf("width %d lacks %q:\n%s", width, want, text)
			}
		}
	}
	if m.workspace.overview.windowRow != -1 {
		t.Fatal("the Bots Overview has a window label")
	}
}

func TestBotsOverviewWithNoProfilesKeepsTheHint(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	listBots(t, m, nil, nil)
	m.rail.Focus(uirail.Selection{Kind: uirail.BotsRow})
	if text := overviewText(m, 100, 40); !strings.Contains(text, "No Bot Profiles yet") {
		t.Fatalf("hint lost:\n%s", text)
	}
}

func TestProfileRowStillShowsItsOwnDetail(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	botsOverviewFixture(t, m, 2)
	m.rail.Focus(uirail.Selection{Kind: uirail.BotProfileRow, Profile: "p0"})
	text := overviewText(m, 100, 40)
	if !strings.Contains(text, "Tool claude") || strings.Contains(text, "Live Bots 1") {
		t.Fatalf("Profile detail lost:\n%s", text)
	}
}

func TestTallBotsOverviewScrolls(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	botsOverviewFixture(t, m, 40)
	y0, y1 := m.bodyYRange()
	h := y1 - y0
	if first := overviewText(m, 100, h); strings.Contains(first, "profile39") || !strings.Contains(first, "below") {
		t.Fatalf("pane shows everything:\n%s", first)
	}
	var last string
	for range 20 {
		m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
		last = overviewText(m, 100, h)
	}
	if !strings.Contains(last, "profile39") || !strings.Contains(last, "0 below") {
		t.Fatalf("never reaches the end:\n%s", last)
	}
	for range 40 {
		m.handleMouse(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp, X: m.layout.width - 3, Y: 5})
		last = overviewText(m, 100, h)
	}
	if !strings.Contains(last, "0 above") {
		t.Fatalf("wheel does not scroll back:\n%s", last)
	}
}

func TestBotsOverviewDrawsNoBrailleUnderCompatibleGlyphs(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	botsOverviewFixture(t, m, 2)
	m.prefs.glyphMode = presentation.GlyphModeBasic
	for _, r := range overviewText(m, 100, 60) {
		if r >= 0x2800 && r <= 0x28FF {
			t.Fatal("Compatible glyph mode drew braille")
		}
	}
}
