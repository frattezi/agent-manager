package ui

import (
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/YoanWai/agent-manager/internal/deps"
	"github.com/YoanWai/agent-manager/internal/git"
	"github.com/YoanWai/agent-manager/internal/keybind"
	"github.com/YoanWai/agent-manager/internal/launch"
	"github.com/YoanWai/agent-manager/internal/sessioncmd"
	"github.com/YoanWai/agent-manager/internal/store"
	"github.com/YoanWai/agent-manager/internal/tmux"
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	uireview "github.com/YoanWai/agent-manager/internal/ui/review"
	uireviewsource "github.com/YoanWai/agent-manager/internal/ui/review/source"
	"github.com/YoanWai/agent-manager/internal/ui/runs"
	uilaunch "github.com/YoanWai/agent-manager/internal/ui/spawn/launch"
	uitheme "github.com/YoanWai/agent-manager/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

const runsPollInterval = 2 * time.Second

// runsState is Fractory's Run registry as last read. Runs are read-only here:
// Fractory writes them, and the rail only lists them.
type runsState struct {
	dir  string
	list []runs.Record
}

type runsLoadedMsg struct {
	dir  string
	list []runs.Record
}

type runsTickMsg struct{}

// runsLoadCmd reads the registry off the update path.
func runsLoadCmd() tea.Cmd {
	return func() tea.Msg {
		dir := runs.Dir()
		if dir == "" {
			return runsLoadedMsg{}
		}
		// An unreadable registry lists nothing; the next poll tries again.
		list, _ := runs.Load(dir)
		return runsLoadedMsg{dir: dir, list: list}
	}
}

func (m *Model) routeRunsMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case runsLoadedMsg:
		if !reflect.DeepEqual(m.workspace.runs.list, msg.list) || m.workspace.runs.dir != msg.dir {
			m.workspace.runs = runsState{dir: msg.dir, list: msg.list}
			m.rebuildRows()
		}
		return routed(m, tea.Tick(runsPollInterval, func(time.Time) tea.Msg { return runsTickMsg{} }))
	case runsTickMsg:
		return routed(m, runsLoadCmd())
	}
	return nil, nil, false
}

func (m *Model) railRuns() []uirail.Run {
	out := make([]uirail.Run, len(m.workspace.runs.list))
	for i, run := range m.workspace.runs.list {
		out[i] = uirail.Run{ID: run.ID, Ticket: run.TicketRef, Status: run.Status, Branch: run.Branch}
	}
	return out
}

// selectedRun is the Run the rail's cursor is on.
func (m *Model) selectedRun() (runs.Record, bool) {
	selection, ok := m.rail.Selected()
	if !ok || selection.Kind != uirail.RunRow {
		return runs.Record{}, false
	}
	for _, run := range m.workspace.runs.list {
		if run.ID == selection.Run {
			return run, true
		}
	}
	return runs.Record{}, false
}

// runIntent runs a rail intent aimed at the Runs section and reports whether
// it took it. Enter opens a Run's log; the keys that change a session refuse,
// since a Run belongs to Fractory.
func (m *Model) runIntent(intent uirail.Intent) (tea.Cmd, bool) {
	if intent.Target.Kind != uirail.RunRow && intent.Target.Kind != uirail.RunsRow {
		return nil, false
	}
	switch intent.Kind {
	case uirail.Quit, uirail.OpenSettings, uirail.Resize, uirail.OpenMessages, uirail.OpenHelp, uirail.Prompt, uirail.NewSession, uirail.NewGroup, uirail.NewConnection:
		return nil, false
	case uirail.Focus, uirail.Attach:
		run, ok := m.selectedRun()
		if !ok {
			return nil, true
		}
		return m.openRunLog(run), true
	case uirail.OpenReview:
		run, ok := m.selectedRun()
		if !ok {
			return nil, true
		}
		return m.openRunDiff(run), true
	default:
		m.reportErr("Runs are read-only: Fractory owns them")
	}
	return nil, true
}

// openRunLog follows the Run's log in a new shell tab. -F keeps trying when a
// queued Run has not created its log yet.
func (m *Model) openRunLog(run runs.Record) tea.Cmd {
	toolName, _ := m.shellTool()
	width, height := m.paneTargetSize()
	m.clearErr()
	m.dispatchSpawn(uilaunch.Request{
		Kind:     uilaunch.Shell,
		ToolName: toolName,
		Name:     sessioncmd.ShellName(toolName, "", newID()[:4], m.workspace.sessions),
		Dir:      filepath.Dir(run.LogPath),
		Pane:     sessioncmd.PaneSize{Width: width, Height: height},
		Plan:     &launch.Plan{Command: "tail -n 200 -F " + tmux.ShellQuote(run.LogPath)},
	})
	return m.nextEffectCmd()
}

func (m *Model) runLegend() (legendSection, bool) {
	selection, ok := m.rail.Selected()
	if !ok || (selection.Kind != uirail.RunRow && selection.Kind != uirail.RunsRow) {
		return legendSection{}, false
	}
	if selection.Kind == uirail.RunsRow {
		return legendSection{Title: "Runs"}, true
	}
	return legendSection{Title: "Run", Pairs: [][2]string{{m.listGlyph(keybind.Open, keybind.Attach), "open log"}}}, true
}

// runContentLines fills the content column for a Run: its facts and, when it
// waits on a human, the question.
func (m *Model) runContentLines(width int) ([]contentLine, bool) {
	run, ok := m.selectedRun()
	if !ok {
		return nil, false
	}
	gutter := "  "
	inner := max(width-len(gutter)*2, 10)
	var lines []contentLine
	add := func(label, value string) {
		if value == "" {
			return
		}
		text := uitheme.LabelStyle().Render(label) + " " + uitheme.ValueStyle().Render(value)
		for _, line := range uitheme.SplitLines(ansi.Wrap(text, inner, "")) {
			lines = append(lines, contentLine{Text: gutter + line})
		}
	}
	add("Ticket", run.TicketRef)
	add("Status", run.Status)
	add("Branch", run.Branch)
	add("PR", run.PRURL)
	add("Worktree", run.WorktreePath)
	add("Log", run.LogPath)
	if question := run.Question(); question != "" {
		lines = append(lines, contentLine{}, contentLine{Text: gutter + uitheme.WarnStyle().Render("Needs a human")})
		for _, line := range uitheme.SplitLines(ansi.Wrap(question, inner, "")) {
			lines = append(lines, contentLine{Text: gutter + line})
		}
	}
	return lines, true
}

const runCommentsSavedNotice = "Comments saved; the factory will pick them up on the next tick"

// runByTarget is the Run a review target id names; a session id names none.
func (m *Model) runByTarget(targetID string) (runs.Record, bool) {
	id, ok := strings.CutPrefix(targetID, uireviewsource.RunTargetPrefix)
	if !ok {
		return runs.Record{}, false
	}
	for _, run := range m.workspace.runs.list {
		if run.ID == id {
			return run, true
		}
	}
	return runs.Record{}, false
}

// runDiffSession stands in for the session a Run's review is keyed under, so
// its drafts live under the Run's own id and no session row is touched.
func (m *Model) runDiffSession() (store.Session, bool) {
	run, ok := m.runByTarget(m.review.SessionID())
	if !ok {
		return store.Session{}, false
	}
	return store.Session{ID: uireviewsource.RunTargetPrefix + run.ID, Name: run.TicketRef, Cwd: run.WorktreePath}, true
}

func (m *Model) runsDirFor(targetID string) string {
	if _, ok := m.runByTarget(targetID); !ok {
		return ""
	}
	return m.workspace.runs.dir
}

// openRunDiff opens Review on the Run's worktree against its base branch.
func (m *Model) openRunDiff(run runs.Record) tea.Cmd {
	if m.services.gitDrv == nil {
		m.reportErr("git not found in PATH, " + deps.Hint("git"))
		return nil
	}
	if run.WorktreePath == "" {
		m.reportErr("this Run has no worktree to review")
		return nil
	}
	m.reviewNav.Set(uireviewsource.Return{Kind: uireviewsource.ReturnList})
	m.clearSurfaces()
	m.base = baseReview
	m.clearErr()
	target := uireview.Target{ID: uireviewsource.RunTargetPrefix + run.ID, Name: run.TicketRef, Cwd: run.WorktreePath}
	return m.reviewLoadCmd(m.review.Open(target, git.ScopeBranch, ""))
}
