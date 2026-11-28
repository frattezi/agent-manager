package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/keybind"
	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	tea "github.com/charmbracelet/bubbletea"
)

func selectAutomationsHeader(t *testing.T, m *Model) {
	t.Helper()
	if !m.rail.Focus(uirail.Selection{Kind: uirail.AutomationsRow}) {
		t.Fatal("no Automations header")
	}
}

func TestAutomationsHeaderShowsTheOverviewAtNarrowAndWideWidths(t *testing.T) {
	m, _, _ := automationFixtures(t)
	checkAutomations(t, m)
	selectAutomationsHeader(t, m)
	for _, width := range []int{40, 110} {
		text := strings.Join(strings.Fields(strings.ReplaceAll(overviewText(m, width, 80), "│", " ")), " ")
		for _, want := range []string{"Window all", "repo", "weekly report", "monday sweep", "disabled", "no upcoming fires", "fresh chore", "never fired", "wrote the report"} {
			if !strings.Contains(text, want) {
				t.Errorf("width %d lacks %q:\n%s", width, want, text)
			}
		}
	}
}

func TestAutomationsOverviewSharesTheWindowAndScroll(t *testing.T) {
	m, _, _ := automationFixtures(t)
	checkAutomations(t, m)
	selectAutomationsHeader(t, m)
	overviewText(m, 100, 40)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("W")})
	if text := overviewText(m, 100, 40); !strings.Contains(text, "Window 24h") || !strings.Contains(text, "no firing in the last 24h") {
		t.Fatalf("after the key:\n%s", text)
	}
	y0, y1 := m.bodyYRange()
	m.handleMouse(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: m.layout.width - 3, Y: y0 + m.workspace.overview.windowRow})
	if text := overviewText(m, 100, 40); !strings.Contains(text, "Window 7d") {
		t.Fatalf("after the click:\n%s", text)
	}
	for i := range 12 {
		id := fmt.Sprintf("chore%d", i)
		addViewAutomation(t, m, id, id, "work", true, time.Now().Add(-time.Hour))
	}
	checkAutomations(t, m)
	h := y1 - y0
	if text := overviewText(m, 100, h); !strings.Contains(text, "below") || strings.Contains(text, "chore11") {
		t.Fatalf("a tall Overview does not scroll:\n%s", text)
	}
	for range 10 {
		m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
		overviewText(m, 100, h)
	}
	if text := overviewText(m, 100, h); !strings.Contains(text, "chore11") || !strings.Contains(text, "0 below") {
		t.Fatalf("paging down never reaches the last Automation:\n%s", text)
	}
	m.handleMouse(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp, X: m.layout.width - 3, Y: 5})
	if text := overviewText(m, 100, h); strings.Contains(text, "0 below") {
		t.Fatalf("the wheel did not scroll:\n%s", text)
	}
}

func TestAutomationAndFiringRowsStillShowTheirDetail(t *testing.T) {
	m, _, _ := automationFixtures(t)
	checkAutomations(t, m)
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: "report"})
	if text := overviewText(m, 100, 40); !strings.Contains(text, "Recent firings, newest first") || strings.Contains(text, "Window") {
		t.Fatalf("an Automation row lost its detail:\n%s", text)
	}
	m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyEnter})
	m.drainEffects(t)
	m.rail.Focus(uirail.Selection{Kind: uirail.FiringRow, Automation: "report", Firing: "run-old"})
	if text := overviewText(m, 100, 40); !strings.Contains(text, "Fired for") || strings.Contains(text, "Window") {
		t.Fatalf("a firing row lost its detail:\n%s", text)
	}
}

func TestAutomationsOverviewEmptyAndCompatible(t *testing.T) {
	fractoryHome(t)
	m := buildModel(t)
	checkAutomations(t, m)
	if m.rail.Focus(uirail.Selection{Kind: uirail.AutomationsRow}) {
		if text := overviewText(m, 100, 40); !strings.Contains(text, "No Automations yet") {
			t.Fatalf("empty:\n%s", text)
		}
	}
	m, _, _ = automationFixtures(t)
	checkAutomations(t, m)
	selectAutomationsHeader(t, m)
	m.prefs.glyphMode = presentation.GlyphModeBasic
	for _, r := range overviewText(m, 100, 60) {
		if r >= 0x2800 && r <= 0x28FF {
			t.Fatal("Compatible glyph mode drew braille")
		}
	}
}

func TestAutomationsHeaderFooterNamesTheOverviewKeys(t *testing.T) {
	m, _, _ := automationFixtures(t)
	checkAutomations(t, m)
	selectAutomationsHeader(t, m)
	legend, _ := m.automationLegend()
	var window, scroll bool
	for _, pair := range legend.Pairs {
		window = window || pair == [2]string{m.listGlyph(keybind.OverviewWindow), "window 24h"}
		scroll = scroll || pair[1] == "scroll"
	}
	if !window || !scroll {
		t.Fatalf("footer pairs = %+v", legend.Pairs)
	}
}
