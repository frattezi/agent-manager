package rail

import (
	"strconv"
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
	// Mode is work or personal; empty when the Run does not say.
	Mode string
	// Reruns is how many Re-runs the Run's Ticket has needed; the row is the
	// Ticket's latest Run.
	Reruns int
	// RerunPending is true while a Re-run request waits for Fractory's tick.
	RerunPending bool
	// CanRerun is true when the Run failed or needs a human, so R asks for a
	// Re-run.
	CanRerun bool
	// Solved is true while the owner has marked the Ticket solved, so . on
	// the row unmarks it.
	Solved bool
	// CanAnswer is true when the Run stopped with a question, so the prompt
	// key opens the answer field.
	CanAnswer bool
	// Answered is true while an answer waits for Fractory's tick.
	Answered bool
}

func (r treeRow) isRun() bool { return r.kind == RunRow || r.kind == RunsRow }

// runRows is the Runs section: a header, then one row per listed Run, or the
// header alone while the section is folded. It is empty when there is
// nothing to list, so a manager with no registry shows no section.
func (m Model) runRows() []treeRow {
	if m.showArchived {
		return nil
	}
	rows := m.listedRunRows()
	// The header stays while finished Tickets are hidden, so H can bring
	// them back.
	if len(rows) == 0 && m.snapshot.RunsFinished == 0 {
		return nil
	}
	header := []treeRow{{kind: RunsRow}}
	if m.honorFolds() && m.hostFolds[RunsFold] {
		return header
	}
	return append(header, rows...)
}

// listedRunRows is one row per Run the search and the filter let through,
// whether or not the section is folded.
func (m Model) listedRunRows() []treeRow {
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
	return rows
}

func (m Model) hasRunsSection() bool {
	return m.rowIndex(Selection{Kind: RunsRow}.key()) >= 0
}

// FinishedRunsAction is what H does to the Runs section, show finished or
// hide finished, empty when no Ticket is finished.
func (m Model) FinishedRunsAction() string {
	switch {
	case m.snapshot.RunsFinished == 0:
		return ""
	case m.snapshot.RunsShowFinished:
		return "hide finished"
	}
	return "show finished"
}

// runsHeaderMenuItems is the header's row menu: fold or unfold, and the
// finished toggle beside it.
func (m Model) runsHeaderMenuItems() []menuItem {
	fold := menuItem{label: "Fold", action: ToggleFold}
	if m.hostFolds[RunsFold] {
		fold.label = "Unfold"
	}
	return append([]menuItem{fold}, m.finishedRunsMenuItems()...)
}

func (m Model) finishedRunsMenuItems() []menuItem {
	switch m.FinishedRunsAction() {
	case "":
		return nil
	case "hide finished":
		return []menuItem{{label: "Hide finished", action: ToggleFinishedRuns}}
	}
	return []menuItem{{label: "Show finished", action: ToggleFinishedRuns}}
}

// runsCount is the header's count: the Runs listed, and the finished
// Tickets when there are any.
func (m Model) runsCount() string {
	count := strconv.Itoa(len(m.listedRunRows()))
	if m.snapshot.RunsFinished > 0 {
		count += ", " + strconv.Itoa(m.snapshot.RunsFinished) + " finished"
	}
	return "(" + count + ")"
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
	marker := r.glyphs().GroupOpenGlyph
	if r.model.rowFolded(treeRow{kind: RunsRow}) {
		marker = r.glyphs().GroupClosedGlyph
	}
	head := pad + r.palette.subtle.Render(marker) + " " + style.Render("Runs") + " " + r.palette.subtle.Render(r.model.runsCount()+" fractory")
	return renderedTreeRow{text: paint(head, width, background), handleX: -1, foldX: ansi.StringWidth(pad)}
}

func (r renderer) renderRun(row treeRow, selected bool, width int, pad, guides, background string) renderedTreeRow {
	state := sessionStatusOf(row.run.Status)
	glyph := lipgloss.NewStyle().Foreground(r.palette.statusColor(state)).Render(r.glyphs().StatusGlyph(state))
	nameStyle := r.palette.value
	if selected {
		nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(r.ctx.Theme.Bright)).Bold(true)
	}
	if marker := r.rerunMarker(row.run); marker != "" {
		glyph += " " + r.palette.subtle.Render(marker)
	}
	status := row.run.Status
	if badge := modeBadge(row.run.Mode); badge != "" {
		status = badge + " " + status
	}
	meta := lipgloss.NewStyle().Foreground(r.palette.statusColor(state)).Render(status)
	lead := pad + guides + glyph + " "
	room := width - railGutter - ansi.StringWidth(lead) - ansi.StringWidth(status) - 2
	name := presentation.EscapeControlsInline(row.run.Ticket)
	if ansi.StringWidth(name) > room {
		name = ansi.Truncate(name, max(room, 0), "…")
	}
	head := lead + r.palette.highlight(name, r.model.search, nameStyle)
	return renderedTreeRow{text: paint(rowColumns(head, meta, width-railGutter), width, background), handleX: -1}
}

func runMenuItems(run Run) []menuItem {
	items := []menuItem{{label: "Open log", action: Attach}}
	if run.CanAnswer {
		answer := menuItem{label: "Answer", action: Prompt}
		if run.Answered {
			answer.label = "Edit answer"
		}
		items = append(items, answer)
	}
	if run.CanRerun {
		items = append(items, menuItem{label: "Re-run", action: Restart})
	}
	solved := menuItem{label: "Mark solved", action: MarkIdle}
	if run.Solved {
		solved.label = "Unmark solved"
	}
	return append(items, solved)
}

// rerunMarker is the Re-run count after the status dot: ↻N in Advanced glyph
// mode, RN in Compatible, with … for N while a request waits for the tick.
func (r renderer) rerunMarker(run Run) string {
	count := ""
	switch {
	case run.RerunPending:
		count = "…"
	case run.Reruns > 0:
		count = strconv.Itoa(run.Reruns)
	default:
		return ""
	}
	if r.ctx.GlyphMode.Effective() == presentation.GlyphModeAdvanced {
		return "↻" + count
	}
	return "R" + count
}

// modeBadge is the one-letter Mode marker: W for work, P for personal.
func modeBadge(mode string) string {
	switch mode {
	case "work":
		return "W"
	case "personal":
		return "P"
	}
	return ""
}
