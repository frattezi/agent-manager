package overview

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/YoanWai/agent-manager/internal/ui/automations"
	"github.com/YoanWai/agent-manager/internal/ui/presentation"
)

func automationSample() Automations {
	return ComputeAutomations([]automations.Entry{
		entry("nightly report", "/src/api", "0 * * * *", true,
			firing("a", "done", 2*time.Hour, "Wrote the report."), firing("b", "failed", 3*time.Hour, "")),
		entry("sweep", "/src/api", "0 * * * *", false, firing("c", "done", time.Hour, "Swept it all away.")),
		entry("loose chore", "", "0 7 1 1 *", true),
	}, WindowAll, now)
}

func TestDrawAutomationsReadsAtNarrowAndWideWidths(t *testing.T) {
	for _, width := range []int{34, 100} {
		d := DrawAutomations(automationSample(), width, "W", presentation.GlyphModeAdvanced, testPalette)
		text := strings.Join(strings.Fields(plain(d)), " ")
		for _, want := range []string{"Window all", "W or click for 24h", "api", "Elsewhere", "nightly report", "Next 24h", "24 fires",
			"Firings", "last 2", "Wrote the report.", "sweep", "disabled", "no upcoming fires", "Swept it all away.", "loose chore", "no fire in the next 24h", "never fired"} {
			if !strings.Contains(text, want) {
				t.Errorf("width %d lacks %q:\n%s", width, want, plain(d))
			}
		}
		for _, line := range d.Lines {
			if w := ansi.StringWidth(line); w > width {
				t.Errorf("width %d: line is %d cells: %q", width, w, ansi.Strip(line))
			}
		}
	}
}

func TestDrawAutomationsCompatibleHasNoBraille(t *testing.T) {
	for _, mode := range []presentation.GlyphMode{presentation.GlyphModeAdvanced, presentation.GlyphModeBasic} {
		for _, r := range plain(DrawAutomations(automationSample(), 80, "W", mode, testPalette)) {
			if r >= 0x2800 && r <= 0x28FF {
				t.Fatalf("mode %v drew braille", mode)
			}
		}
	}
}

func TestDrawAutomationsEmptyStates(t *testing.T) {
	if text := plain(DrawAutomations(Automations{Window: WindowAll}, 60, "W", presentation.GlyphModeAdvanced, testPalette)); !strings.Contains(text, "No Automations yet") {
		t.Errorf("empty: %q", text)
	}
	one := ComputeAutomations([]automations.Entry{entry("a", "/src/a", "0 7 1 1 *", true, firing("x", "done", 40*24*time.Hour, "old"))}, Window7d, now)
	if text := plain(DrawAutomations(one, 80, "W", presentation.GlyphModeAdvanced, testPalette)); !strings.Contains(text, "no firing in the last 7d") || strings.Contains(text, "old") {
		t.Errorf("window past the firings: %q", text)
	}
}
