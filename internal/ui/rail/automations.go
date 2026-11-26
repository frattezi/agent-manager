package rail

import (
	"strconv"
	"strings"

	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// NeverFired is what an Automation row says before its first firing.
const NeverFired = "never fired"

// Automation is the copied part of a stored Automation the rail lists, with
// its firings. Root formats every time, so the rail reads no clock.
type Automation struct {
	ID   string
	Name string
	// Mode is work or personal.
	Mode    string
	Enabled bool
	// Last is how the latest firing went, NeverFired before the first.
	Last string
	// LastStatus is the latest firing's record status, empty before the
	// first; it colours the row's dot.
	LastStatus string
	// Next is when it fires next, empty when disabled or unreadable.
	Next    string
	Firings []Firing
}

// Firing is one Automation Run, listed under its Automation newest first.
type Firing struct {
	ID string
	// When is the instant it fired for.
	When     string
	Status   string
	Duration string
}

// AutomationsFold is the Automations section's fold, saved beside RunsFold
// under the empty host.
var AutomationsFold = HostFold{Path: "automations"}

func (r treeRow) isAutomation() bool {
	return r.kind == AutomationsRow || r.kind == AutomationRow || r.kind == FiringRow
}

// readOnly reports a row the manager lists but does not own.
func (r treeRow) readOnly() bool { return r.isRun() || r.isAutomation() }

// automationRows is the Automations section: a header, then each listed
// Automation with its firings under it. A store with no Automations shows
// no section.
func (m Model) automationRows() []treeRow {
	if m.showArchived {
		return nil
	}
	rows := m.listedAutomationRows()
	if len(rows) == 0 {
		return nil
	}
	header := []treeRow{{kind: AutomationsRow}}
	if m.honorFolds() && m.hostFolds[AutomationsFold] {
		return header
	}
	return append(header, rows...)
}

func (m Model) listedAutomationRows() []treeRow {
	query := strings.ToLower(strings.TrimSpace(m.search))
	var rows []treeRow
	for _, automation := range m.snapshot.Automations {
		if m.filter == statusFilterAttention && automation.LastStatus != "failed" && automation.LastStatus != "needs-human" {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(automation.Name+" "+automation.Mode+" "+automation.Last), query) {
			continue
		}
		rows = append(rows, treeRow{kind: AutomationRow, depth: 1, automation: automation})
		for _, firing := range automation.Firings {
			rows = append(rows, treeRow{kind: FiringRow, depth: 2, automation: automation, firing: firing})
		}
	}
	return rows
}

func (m Model) hasAutomationsSection() bool {
	return m.rowIndex(Selection{Kind: AutomationsRow}.key()) >= 0
}

func (m Model) automationsCount() string {
	return "(" + strconv.Itoa(len(m.snapshot.Automations)) + ")"
}

func (m Model) automationsHeaderMenuItems() []menuItem {
	fold := menuItem{label: "Fold", action: ToggleFold}
	if m.hostFolds[AutomationsFold] {
		fold.label = "Unfold"
	}
	return []menuItem{fold}
}

func automationMenuItems(row treeRow) []menuItem {
	if row.kind == AutomationRow && len(row.automation.Firings) == 0 {
		return nil
	}
	label := "Open log"
	if row.kind == AutomationRow {
		label = "Open latest log"
	}
	return []menuItem{{label: label, action: Attach}}
}

func (r renderer) renderAutomationsHeader(selected bool, width int, pad, background string) renderedTreeRow {
	style := r.palette.subtle.Bold(true)
	if selected {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color(r.ctx.Theme.Bright)).Bold(true)
	}
	marker := r.glyphs().GroupOpenGlyph
	if r.model.rowFolded(treeRow{kind: AutomationsRow}) {
		marker = r.glyphs().GroupClosedGlyph
	}
	head := pad + r.palette.subtle.Render(marker) + " " + style.Render("Automations") + " " + r.palette.subtle.Render(r.model.automationsCount())
	return renderedTreeRow{text: paint(head, width, background), handleX: -1, foldX: ansi.StringWidth(pad)}
}

// automationTimes is the row's second line: how the last firing went, and
// when it fires next or that it is disabled.
func automationTimes(automation Automation) string {
	switch {
	case !automation.Enabled:
		return automation.Last + " · disabled"
	case automation.Next != "":
		return automation.Last + " · next " + automation.Next
	}
	return automation.Last
}

// renderAutomation is two lines: the dot, the name and the Mode, then the
// last outcome and the next fire time under the name.
func (r renderer) renderAutomation(row treeRow, selected bool, width int, pad, guides, trail, background string) renderedTreeRow {
	automation := row.automation
	state := sessionStatusOf(automation.LastStatus)
	color := r.palette.statusColor(state)
	nameStyle := r.palette.value
	if !automation.Enabled {
		nameStyle = r.palette.subtle
		color = r.palette.statusColor("idle")
	}
	if selected {
		nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(r.ctx.Theme.Bright)).Bold(true)
	}
	glyph := lipgloss.NewStyle().Foreground(color).Render(r.glyphs().StatusGlyph(state))
	first := r.readOnlyRow(pad+guides+glyph+" ", automation.Name, modeBadge(automation.Mode), r.palette.subtle, nameStyle, width, background)
	lead := pad + trail
	times := ansi.Truncate(automationTimes(automation), max(width-railGutter-ansi.StringWidth(lead), 0), "…")
	first.text += "\n" + paint(lead+lipgloss.NewStyle().Foreground(color).Render(times), width, background)
	return first
}

func (r renderer) renderFiring(row treeRow, selected bool, width int, pad, guides, background string) renderedTreeRow {
	firing := row.firing
	state := sessionStatusOf(firing.Status)
	color := r.palette.statusColor(state)
	nameStyle := r.palette.value
	if selected {
		nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(r.ctx.Theme.Bright)).Bold(true)
	}
	glyph := lipgloss.NewStyle().Foreground(color).Render(r.glyphs().StatusGlyph(state))
	// The dot carries the outcome; the narrow rail keeps room for the
	// instant, and the detail column spells the outcome out.
	return r.readOnlyRow(pad+guides+glyph+" ", firing.When, firing.Duration, lipgloss.NewStyle().Foreground(color), nameStyle, width, background)
}

// readOnlyRow lays a lead, a name cut to fit and a right-aligned meta on
// one line, the shape a Run row has.
func (r renderer) readOnlyRow(lead, name, meta string, metaStyle, nameStyle lipgloss.Style, width int, background string) renderedTreeRow {
	room := width - railGutter - ansi.StringWidth(lead) - ansi.StringWidth(meta) - 2
	name = presentation.EscapeControlsInline(name)
	if ansi.StringWidth(name) > room {
		name = ansi.Truncate(name, max(room, 0), "…")
	}
	head := lead + r.palette.highlight(name, r.model.search, nameStyle)
	return renderedTreeRow{text: paint(rowColumns(head, metaStyle.Render(meta), width-railGutter), width, background), handleX: -1}
}
