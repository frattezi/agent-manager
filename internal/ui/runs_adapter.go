package ui

import (
	"cmp"
	"maps"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/YoanWai/agent-manager/internal/deps"
	"github.com/YoanWai/agent-manager/internal/git"
	"github.com/YoanWai/agent-manager/internal/keybind"
	"github.com/YoanWai/agent-manager/internal/launch"
	"github.com/YoanWai/agent-manager/internal/sessioncmd"
	"github.com/YoanWai/agent-manager/internal/status"
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
	// requested holds the ids of Runs with a Re-run request file.
	requested map[string]bool
	// solved holds the ids of Runs whose Ticket the owner marked solved.
	solved map[string]bool
	// answers maps the ids of Runs with an answer waiting for the next tick
	// to its text.
	answers map[string]string
	// host is Fractory's host.json, nil when absent or invalid.
	host *runs.HostState
	// showFinished lists the finished Tickets too; H toggles it.
	showFinished bool
}

type runsLoadedMsg struct {
	dir       string
	list      []runs.Record
	requested map[string]bool
	solved    map[string]bool
	answers   map[string]string
	host      *runs.HostState
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
		return runsLoadedMsg{dir: dir, list: list, requested: runs.Requested(dir), solved: runs.Solved(dir), answers: runs.Answers(dir), host: runs.LoadHost(dir)}
	}
}

func (m *Model) routeRunsMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case runsLoadedMsg:
		next := runsState{dir: msg.dir, list: msg.list, requested: msg.requested, solved: msg.solved, answers: msg.answers, host: msg.host, showFinished: m.workspace.runs.showFinished}
		if !reflect.DeepEqual(m.workspace.runs, next) {
			m.workspace.runs = next
			m.rebuildRows()
		}
		return routed(m, tea.Tick(runsPollInterval, func(time.Time) tea.Msg { return runsTickMsg{} }))
	case runsTickMsg:
		return routed(m, runsLoadCmd())
	case runsAnswerSavedMsg:
		m.answerSaved(msg)
		return routed(m, nil)
	}
	return nil, nil, false
}

// railRuns is one rail row per listed Ticket, and how many Tickets are
// finished.
func (m *Model) railRuns() ([]uirail.Run, int) {
	tickets, finished := runs.Listed(m.runTickets(), m.workspace.runs.showFinished)
	out := make([]uirail.Run, len(tickets))
	for i, ticket := range tickets {
		run := ticket.Latest()
		out[i] = uirail.Run{
			ID: run.ID, Ticket: run.TicketRef, Status: run.Status, Branch: run.Branch, Mode: run.EffectiveMode(),
			Reruns: ticket.Reruns, RerunPending: ticket.Pending, CanRerun: run.CanRerun(), Solved: ticket.Solved,
			CanAnswer: run.CanAnswer(), Answered: m.workspace.runs.answers[run.ID] != "",
		}
	}
	return out, finished
}

// runTickets is the registry grouped by Ticket: one rail row each.
func (m *Model) runTickets() []runs.Ticket {
	return runs.Tickets(m.workspace.runs.list, m.workspace.runs.requested, m.workspace.runs.solved)
}

// selectedTicket is the Ticket the rail's cursor is on.
func (m *Model) selectedTicket() (runs.Ticket, bool) {
	selection, ok := m.rail.Selected()
	if !ok || selection.Kind != uirail.RunRow {
		return runs.Ticket{}, false
	}
	for _, ticket := range m.runTickets() {
		if ticket.Latest().ID == selection.Run {
			return ticket, true
		}
	}
	return runs.Ticket{}, false
}

// requestRerun asks Fractory to re-run the selected Ticket's latest Run when it
// failed or needs a human. A request already on disk is left as it is.
func (m *Model) requestRerun() {
	ticket, ok := m.selectedTicket()
	if !ok {
		return
	}
	run := ticket.Latest()
	if !run.CanRerun() {
		m.reportErr("Only a failed or needs-human Run can be re-run")
		return
	}
	if m.workspace.runs.dir == "" {
		m.reportErr("no Run registry to write a Re-run request to")
		return
	}
	wrote, err := runs.RequestRerun(m.workspace.runs.dir, run.ID, time.Now())
	if err != nil {
		m.reportErr("could not request a Re-run: " + err.Error())
		return
	}
	m.clearErr()
	if wrote {
		if m.workspace.runs.requested == nil {
			m.workspace.runs.requested = map[string]bool{}
		}
		m.workspace.runs.requested[run.ID] = true
		m.rebuildRows()
	}
}

// toggleSolved marks the selected Ticket solved, or unmarks it when it is.
// The mark is named for the latest Run, so a newer Run lists it again.
func (m *Model) toggleSolved() {
	ticket, ok := m.selectedTicket()
	if !ok {
		return
	}
	if m.workspace.runs.dir == "" {
		m.reportErr("no Run registry to write a solved mark to")
		return
	}
	id := ticket.Latest().ID
	var err error
	if ticket.Solved {
		_, err = runs.UnmarkSolved(m.workspace.runs.dir, id)
	} else {
		_, err = runs.MarkSolved(m.workspace.runs.dir, id, time.Now())
	}
	if err != nil {
		m.reportErr("could not change the solved mark: " + err.Error())
		return
	}
	m.clearErr()
	solved := maps.Clone(m.workspace.runs.solved)
	if solved == nil {
		solved = map[string]bool{}
	}
	if ticket.Solved {
		delete(solved, id)
	} else {
		solved[id] = true
	}
	m.workspace.runs.solved = solved
	m.rebuildRows()
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
	case uirail.Prompt:
		if ticket, ok := m.selectedTicket(); ok && ticket.Latest().CanAnswer() {
			m.openAnswer(ticket.Latest())
			return nil, true
		}
		return nil, false
	case uirail.Quit, uirail.OpenSettings, uirail.Resize, uirail.OpenMessages, uirail.OpenHelp, uirail.NewSession, uirail.NewGroup, uirail.NewConnection:
		return nil, false
	case uirail.Focus, uirail.Attach:
		run, ok := m.selectedRun()
		if !ok {
			return nil, true
		}
		return m.openRunLog(run), true
	case uirail.Restart:
		m.requestRerun()
		return nil, true
	case uirail.MarkIdle:
		m.toggleSolved()
		return nil, true
	case uirail.ToggleFinishedRuns:
		m.workspace.runs.showFinished = !m.workspace.runs.showFinished
		m.rebuildRows()
		return nil, true
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

var nonNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// runLogName names a Run's log tab from the Run, so the same Run always finds
// the same tab: log-<short ticket ref>-<first 4 of the run id>.
func runLogName(run runs.Record) string {
	ref := run.TicketRef
	if i := strings.LastIndexAny(ref, ":/"); i >= 0 {
		ref = ref[i+1:]
	}
	ref = strings.Trim(nonNameChars.ReplaceAllString(ref, "-"), "-")
	return "log-" + ref + "-" + run.ID[:min(4, len(run.ID))]
}

// openRunLog follows the Run's log in a shell tab.
func (m *Model) openRunLog(run runs.Record) tea.Cmd {
	return m.openLogTab(runLogName(run), run.LogPath)
}

// openLogTab follows a log in a shell tab named name. -F keeps trying when
// the log has yet to be created. A live tab already named so is focused
// instead of spawning another.
func (m *Model) openLogTab(name, logPath string) tea.Cmd {
	for _, sess := range m.workspace.sessions {
		if sess.Name != name || sess.Archived || sess.Status == status.Dead {
			continue
		}
		m.clearErr()
		if m.focusSession(sess.ID) {
			_, cmd := m.focusSelected()
			return cmd
		}
		return m.attachCmd(sess.ID)
	}
	toolName, _ := m.shellTool()
	width, height := m.paneTargetSize()
	m.clearErr()
	m.dispatchSpawn(uilaunch.Request{
		Kind:     uilaunch.Shell,
		ToolName: toolName,
		Name:     name,
		Dir:      filepath.Dir(logPath),
		Pane:     sessioncmd.PaneSize{Width: width, Height: height},
		Plan:     &launch.Plan{Command: "tail -n 200 -F " + tmux.ShellQuote(logPath)},
	})
	return m.nextEffectCmd()
}

func (m *Model) runLegend() (legendSection, bool) {
	selection, ok := m.rail.Selected()
	if !ok || (selection.Kind != uirail.RunRow && selection.Kind != uirail.RunsRow) {
		return legendSection{}, false
	}
	var finished [][2]string
	if action := m.rail.FinishedRunsAction(); action != "" {
		finished = [][2]string{{m.listGlyph(keybind.FinishedRuns), action}}
	}
	if selection.Kind == uirail.RunsRow {
		fold, mark := "fold", m.glyphs().GroupOpenGlyph
		if m.rail.SelectionCollapsed(selection) {
			fold, mark = "unfold", m.glyphs().GroupClosedGlyph
		}
		pairs := [][2]string{{m.listGlyph(keybind.Open), fold}}
		if !m.prefs.mouseDisabled {
			pairs = append(pairs, [2]string{"click " + mark, fold})
		}
		return legendSection{Title: "Runs", Pairs: append(pairs, finished...)}, true
	}
	pairs := [][2]string{{m.listGlyph(keybind.Open, keybind.Attach), "open log"}}
	if ticket, ok := m.selectedTicket(); ok && ticket.Latest().CanRerun() {
		pairs = append(pairs, [2]string{m.listGlyph(keybind.Restart), "re-run"})
	}
	if ticket, ok := m.selectedTicket(); ok && ticket.Latest().CanAnswer() {
		pairs = append(pairs, [2]string{m.listGlyph(keybind.Prompt), m.answerAction(ticket.Latest())})
	}
	if ticket, ok := m.selectedTicket(); ok {
		pairs = append(pairs, [2]string{m.listGlyph(keybind.MarkIdle), solvedAction(ticket)})
	}
	return legendSection{Title: "Run", Pairs: append(pairs, finished...)}, true
}

// solvedAction is what the mark-handled key does to the Ticket.
func solvedAction(ticket runs.Ticket) string {
	if ticket.Solved {
		return "unmark solved"
	}
	return "mark solved"
}

// runContentLines fills the content column for a Run: its facts and, when it
// waits on a human, the question.
func (m *Model) runContentLines(width int) ([]contentLine, bool) {
	gutter := "  "
	inner := max(width-len(gutter)*2, 10)
	var lines []contentLine
	if held := runs.HeldLine(m.workspace.runs.host, time.Now()); held != "" {
		for _, line := range uitheme.SplitLines(ansi.Wrap(held, inner, "")) {
			lines = append(lines, contentLine{Text: gutter + uitheme.WarnStyle().Render(line)})
		}
		lines = append(lines, contentLine{})
	}
	if selection, ok := m.rail.Selected(); ok && selection.Kind == uirail.RunsRow && len(lines) > 0 {
		return lines, true
	}
	run, ok := m.selectedRun()
	if !ok {
		return nil, false
	}
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
	orUnknown := func(value string) string {
		if value == "" {
			return runs.Unknown
		}
		return value
	}
	rules := run.Rules()
	agents := strings.Join(run.Agents, ", ")
	lines = append(lines, contentLine{})
	add("Mode", orUnknown(run.EffectiveMode()))
	add("Flow", orUnknown(run.EffectiveFlow()))
	add("Agents", orUnknown(agents))
	lines = append(lines, contentLine{Text: gutter + uitheme.LabelStyle().Render("Workflow rules")})
	add("Merge", rules.Merge)
	add("Ticket Gate", rules.TicketGate)
	add("Allowed agents", rules.Agents)
	if question := run.Question(); question != "" {
		lines = append(lines, contentLine{}, contentLine{Text: gutter + uitheme.WarnStyle().Render("Needs a human")})
		for _, line := range uitheme.SplitLines(ansi.Wrap(question, inner, "")) {
			lines = append(lines, contentLine{Text: gutter + line})
		}
	}
	if run.CanAnswer() {
		lines = append(lines, m.answerLines(run, gutter, inner)...)
	}
	if ticket, ok := m.selectedTicket(); ok && len(ticket.Runs) > 1 {
		lines = append(lines, contentLine{}, contentLine{Text: gutter + uitheme.LabelStyle().Render("Runs of this Ticket, newest first")})
		for _, past := range ticket.Runs {
			add(cmp.Or(past.Trigger.Kind, "run")+" "+runClock(past), runResult(past)+" "+past.LogPath)
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

// runClock is when a history entry was created, in the local time zone.
func runClock(run runs.Record) string {
	at := run.CreatedAt
	if at.IsZero() {
		at = run.UpdatedAt
	}
	return at.Local().Format("2006-01-02 15:04")
}

// runResult is a history entry's status, with the outcome when it differs.
func runResult(run runs.Record) string {
	if run.Result != nil && run.Result.Outcome != "" && run.Result.Outcome != run.Status {
		return run.Status + " (" + run.Result.Outcome + ")"
	}
	return run.Status
}
