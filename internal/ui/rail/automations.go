package rail

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Automation is the copied part of a stored Automation the rail lists, with
// the words its row shows already worked out by the root.
type Automation struct {
	ID   string
	Name string
	// Mode is work or personal; empty when the configuration cannot be read.
	Mode    string
	Enabled bool
	// Last is how the newest firing went, empty when it never fired.
	Last string
	// LastStatus is the newest firing's record status, which paints its mark.
	LastStatus string
	// Next is when it fires next, empty when disabled or unschedulable.
	Next string
	// Firings are its recent firings, newest first.
	Firings []Firing
}

// Firing is one Automation Run the rail lists under its Automation.
type Firing struct {
	ID string
	// When is the instant it fired for, as the row shows it.
	When string
	// Status is the record's status; Outcome is what the row says of it.
	Status  string
	Outcome string
}

// NeverFired is what an Automation row says before its first firing.
const NeverFired = "never fired"

func (r treeRow) isAutomation() bool {
	return r.kind == AutomationsRow || r.kind == AutomationRow || r.kind == FiringRow
}

// automationRows is the Automations section: a header, one row per listed
// Automation and, under an unfolded one, its firings. It is empty with no
// Automation to list.
func (m Model) automationRows() []treeRow {
	if m.showArchived {
		return nil
	}
	listed := m.listedAutomations()
	if len(listed) == 0 {
		return nil
	}
	rows := []treeRow{{kind: AutomationsRow}}
	if m.honorFolds() && m.hostFolds[AutomationsFold] {
		return rows
	}
	for _, automation := range listed {
		rows = append(rows, treeRow{kind: AutomationRow, depth: 1, auto: automation})
		if !m.openAutomations[automation.ID] {
			continue
		}
		for _, firing := range automation.Firings {
			rows = append(rows, treeRow{kind: FiringRow, depth: 2, auto: automation, fire: firing})
		}
	}
	return rows
}

// listedAutomations is every Automation the search and the filter let
// through; the attention filter keeps the ones whose last firing failed.
func (m Model) listedAutomations() []Automation {
	query := strings.ToLower(strings.TrimSpace(m.search))
	var listed []Automation
	for _, automation := range m.snapshot.Automations {
		if m.filter == statusFilterAttention && automation.LastStatus != "failed" && automation.LastStatus != "needs-human" {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(automation.Name+" "+automation.Mode+" "+automation.Last), query) {
			continue
		}
		listed = append(listed, automation)
	}
	return listed
}

var newAutomation = menuItem{label: "New Automation", action: NewAutomation}

func automationsHeaderMenuItems(folded bool) []menuItem {
	fold := menuItem{label: "Fold", action: ToggleFold}
	if folded {
		fold.label = "Unfold"
	}
	return menuSections([]menuItem{fold}, []menuItem{newAutomation})
}

func automationMenuItems(automation Automation, folded bool) []menuItem {
	fold := menuItem{label: "Hide firings", action: ToggleFold}
	if folded {
		fold.label = "Show firings"
	}
	view := []menuItem{fold}
	if len(automation.Firings) > 0 {
		view = append(view, menuItem{label: "Open last log", action: Attach})
	}
	toggle := menuItem{label: "Disable", action: ToggleEnabled}
	if !automation.Enabled {
		toggle.label = "Enable"
	}
	manage := []menuItem{{label: "Edit", action: RenameAction}, toggle, {label: "Run now", action: RunNow}, newAutomation}
	return menuSections(view, manage, []menuItem{{label: "Delete", action: Delete, danger: true}})
}

// automationIntent is an Automation action, which only an Automation row
// takes; anywhere else the key does nothing.
func (m *Model) automationIntent(kind ActionKind) Decision {
	if row, ok := m.row(); !ok || row.kind != AutomationRow {
		return Decision{Consumed: true}
	}
	return m.intent(kind)
}

// firingIntent is a firing action, which only a firing row takes; anywhere
// else the key does nothing.
func (m *Model) firingIntent(kind ActionKind) Decision {
	if row, ok := m.row(); !ok || row.kind != FiringRow {
		return Decision{Consumed: true}
	}
	return m.intent(kind)
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
	count := "(" + strconv.Itoa(len(r.model.listedAutomations())) + ")"
	head := pad + r.palette.subtle.Render(marker) + " " + style.Render("Automations") + " " + r.palette.subtle.Render(count)
	return renderedTreeRow{text: paint(head, width, background), handleX: -1, foldX: ansi.StringWidth(pad)}
}

// renderAutomation is two lines: the fold mark, the last firing's mark, the
// name and the Mode, then the last outcome and the next fire time, which a
// narrow rail has no room for beside the name.
func (r renderer) renderAutomation(row treeRow, selected bool, width int, pad, guides, trail, background string) renderedTreeRow {
	automation := row.auto
	marker := r.glyphs().GroupOpenGlyph
	if r.model.rowFolded(row) {
		marker = r.glyphs().GroupClosedGlyph
	}
	state := sessionStatusOf(automation.LastStatus)
	glyph := lipgloss.NewStyle().Foreground(r.palette.statusColor(state)).Render(r.glyphs().StatusGlyph(state))
	nameStyle := r.palette.value
	if !automation.Enabled {
		nameStyle = r.palette.muted
	}
	if selected {
		nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(r.ctx.Theme.Bright)).Bold(true)
	}
	badge := modeBadge(automation.Mode)
	parts := []string{cmp.Or(automation.Last, NeverFired)}
	if !automation.Enabled {
		parts = append(parts, "disabled")
	} else if automation.Next != "" {
		parts = append(parts, "next "+automation.Next)
	}
	status := strings.Join(parts, " · ")
	metaStyle := lipgloss.NewStyle().Foreground(r.palette.statusColor(state))
	if !automation.Enabled {
		metaStyle = r.palette.muted
	}
	lead := pad + guides + r.palette.subtle.Render(marker) + " " + glyph + " "
	room := width - railGutter - ansi.StringWidth(lead) - ansi.StringWidth(badge) - 2
	name := presentation.EscapeControlsInline(automation.Name)
	if ansi.StringWidth(name) > room {
		name = ansi.Truncate(name, max(room, 0), "…")
	}
	head := lead + r.palette.highlight(name, r.model.search, nameStyle)
	first := paint(rowColumns(head, r.palette.subtle.Render(badge), width-railGutter), width, background)
	indent := pad + trail
	// The second line has no menu button, so it takes the button's column
	// as well as the gutter.
	full := width
	if !r.ctx.MouseDisabled {
		full += menuButtonWidth
	}
	status = ansi.Truncate(status, max(full-ansi.StringWidth(indent)-1, 0), "…")
	second := paint(indent+metaStyle.Render(status), full, background)
	return renderedTreeRow{text: first + "\n" + second, handleX: -1, foldX: ansi.StringWidth(pad + guides)}
}

// renderFiring is the firing's mark and instant, its outcome on the right.
func (r renderer) renderFiring(row treeRow, selected bool, width int, pad, guides, background string) renderedTreeRow {
	state := sessionStatusOf(row.fire.Status)
	glyph := lipgloss.NewStyle().Foreground(r.palette.statusColor(state)).Render(r.glyphs().StatusGlyph(state))
	nameStyle := r.palette.value
	if selected {
		nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(r.ctx.Theme.Bright)).Bold(true)
	}
	outcome := presentation.EscapeControlsInline(row.fire.Outcome)
	meta := lipgloss.NewStyle().Foreground(r.palette.statusColor(state)).Render(outcome)
	head := pad + guides + glyph + " " + nameStyle.Render(row.fire.When)
	return renderedTreeRow{text: paint(rowColumns(head, meta, width-railGutter), width, background), handleX: -1}
}
