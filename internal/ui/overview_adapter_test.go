package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/keybind"
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
