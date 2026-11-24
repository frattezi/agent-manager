package rail

import (
	"strings"

	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Run is the copied part of a Fractory Run the rail lists. Runs are read-only:
// the rail never changes one.
type Run struct {
	ID     string
	Ticket string
	// Status is the Run's own: queued, running, done, failed or needs-human.
	Status string
	Branch string
}

func (r treeRow) isRun() bool { return r.kind == RunRow || r.kind == RunsRow }

// runRows is the Runs section: a header, then one row per listed Run. It is
// empty when there is nothing to list, so a manager with no registry shows
// no section.
func (m Model) runRows() []treeRow {
	if m.showArchived {
		return nil
	}
	query := strings.ToLower(strings.TrimSpace(m.search))
	var rows []treeRow
	for _, run := range m.snapshot.Runs {
		if m.filter == statusFilterAttention && run.Status != "needs-human" && run.Status != "failed" {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(run.Ticket+" "+run.Branch+" "+run.Status), query) {
			continue
		}
		rows = append(rows, treeRow{kind: RunRow, depth: 1, run: run})
	}
	if len(rows) == 0 {
		return nil
	}
	return append([]treeRow{{kind: RunsRow}}, rows...)
}

// sessionStatusOf maps a Run's status onto the session status that paints the
// same way, so a Run reads like an agent: working, waiting, finished, errored.
func sessionStatusOf(run string) string {
	switch run {
	case "running":
		return "working"
	case "needs-human":
		return "waiting"
	case "done":
		return "finished"
	case "failed":
		return "errored"
	}
	return "idle"
}

func (r renderer) renderRunsHeader(selected bool, width int, pad, background string) renderedTreeRow {
	style := r.palette.subtle.Bold(true)
	if selected {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color(r.ctx.Theme.Bright)).Bold(true)
	}
	head := pad + style.Render("Runs") + " " + r.palette.subtle.Render("fractory")
	return renderedTreeRow{text: paint(head, width, background), handleX: -1}
}

func (r renderer) renderRun(row treeRow, selected bool, width int, pad, guides, background string) renderedTreeRow {
	state := sessionStatusOf(row.run.Status)
	glyph := lipgloss.NewStyle().Foreground(r.palette.statusColor(state)).Render(r.glyphs().StatusGlyph(state))
	nameStyle := r.palette.value
	if selected {
		nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(r.ctx.Theme.Bright)).Bold(true)
	}
	meta := lipgloss.NewStyle().Foreground(r.palette.statusColor(state)).Render(row.run.Status)
	lead := pad + guides + glyph + " "
	room := width - railGutter - ansi.StringWidth(lead) - ansi.StringWidth(row.run.Status) - 2
	name := presentation.EscapeControlsInline(row.run.Ticket)
	if ansi.StringWidth(name) > room {
		name = ansi.Truncate(name, max(room, 0), "…")
	}
	head := lead + r.palette.highlight(name, r.model.search, nameStyle)
	return renderedTreeRow{text: paint(rowColumns(head, meta, width-railGutter), width, background), handleX: -1}
}

func runMenuItems() []menuItem {
	return []menuItem{{label: "Open log", action: Attach}}
}
