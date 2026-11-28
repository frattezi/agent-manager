package ui

import (
	"reflect"
	"strings"
	"time"

	"github.com/YoanWai/agent-manager/internal/keybind"
	"github.com/YoanWai/agent-manager/internal/ui/automations"
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	"github.com/YoanWai/agent-manager/internal/ui/runs"
	uitheme "github.com/YoanWai/agent-manager/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// showAutomations takes what a check read and lists it, rebuilding the rail
// only when a row reads differently.
func (m *Model) showAutomations(rows []automations.Row, records []automations.Record) {
	m.workspace.automations.entries = automations.Entries(rows, records)
	shown := railAutomations(m.workspace.automations.entries, time.Now())
	if !reflect.DeepEqual(shown, m.workspace.automations.shown) {
		m.workspace.automations.shown = shown
		m.rebuildRows()
	}
}

func railAutomations(entries []automations.Entry, now time.Time) []uirail.Automation {
	out := make([]uirail.Automation, len(entries))
	for i, entry := range entries {
		row := uirail.Automation{ID: entry.ID, Name: entry.Name, Mode: entry.Config.Mode, Enabled: entry.Enabled}
		if last, ok := entry.Last(); ok {
			row.Last, row.LastStatus = last.Outcome(), last.Status
		}
		if entry.Err != nil {
			row.Last = "unreadable"
		}
		if next, ok := entry.Next(now); ok {
			row.Next = nextLabel(next, now)
		}
		for _, record := range entry.Firings {
			row.Firings = append(row.Firings, uirail.Firing{ID: record.ID, When: record.FiredFor.Local().Format("Jan 2 15:04"), Status: record.Status, Outcome: record.Outcome()})
		}
		out[i] = row
	}
	return out
}

// nextLabel is a fire time as short as the row can say it: the clock today,
// the weekday within the week, the day of the year after that.
func nextLabel(next, now time.Time) string {
	next, now = next.Local(), now.Local()
	switch {
	case next.YearDay() == now.YearDay() && next.Year() == now.Year():
		return next.Format("15:04")
	case next.Sub(now) < 6*24*time.Hour:
		return next.Format("Mon 15:04")
	}
	return next.Format("Jan 2")
}

func firingClock(at time.Time) string { return at.Local().Format("2006-01-02 15:04") }

// selectedAutomation is the Automation the cursor is on, or whose firing it
// is on.
func (m *Model) selectedAutomation() (automations.Entry, bool) {
	selection, ok := m.rail.Selected()
	if !ok || (selection.Kind != uirail.AutomationRow && selection.Kind != uirail.FiringRow) {
		return automations.Entry{}, false
	}
	for _, entry := range m.workspace.automations.entries {
		if entry.ID == selection.Automation {
			return entry, true
		}
	}
	return automations.Entry{}, false
}

// selectedFiring is the firing the cursor is on.
func (m *Model) selectedFiring() (automations.Record, bool) {
	selection, ok := m.rail.Selected()
	if !ok || selection.Kind != uirail.FiringRow {
		return automations.Record{}, false
	}
	entry, ok := m.selectedAutomation()
	if !ok {
		return automations.Record{}, false
	}
	for _, record := range entry.Firings {
		if record.ID == selection.Firing {
			return record, true
		}
	}
	return automations.Record{}, false
}

func isAutomationRow(kind uirail.RowKind) bool {
	return kind == uirail.AutomationsRow || kind == uirail.AutomationRow || kind == uirail.FiringRow
}

// automationIntent runs a rail intent aimed at the Automations section and
// reports whether it took it. New Automation opens the dialog from any row.
// Open and attach on a firing open its log, and on an Automation its newest
// firing's; a firing opens a Debug Session, an Automation row takes its own
// actions, and the rest refuse.
func (m *Model) automationIntent(intent uirail.Intent) (tea.Cmd, bool) {
	if intent.Kind == uirail.NewAutomation {
		m.openAutomationDialog(nil)
		return nil, true
	}
	if !isAutomationRow(intent.Target.Kind) {
		return nil, false
	}
	switch intent.Kind {
	case uirail.Quit, uirail.OpenSettings, uirail.Resize, uirail.OpenMessages, uirail.OpenHelp, uirail.Prompt, uirail.NewSession, uirail.NewGroup, uirail.NewConnection:
		return nil, false
	case uirail.Focus, uirail.Attach:
		if intent.Target.Kind == uirail.FiringRow {
			if record, ok := m.selectedFiring(); ok {
				return m.openFiringLog(record), true
			}
			return nil, true
		}
		entry, ok := m.selectedAutomation()
		if !ok {
			return nil, true
		}
		last, ok := entry.Last()
		if !ok {
			m.reportErr("this Automation has " + uirail.NeverFired)
			return nil, true
		}
		return m.openFiringLog(last), true
	case uirail.DebugSession:
		entry, entryOK := m.selectedAutomation()
		if record, ok := m.selectedFiring(); ok && entryOK {
			return m.openDebugSession(entry, record), true
		}
		return nil, true
	default:
		if intent.Target.Kind == uirail.AutomationRow && m.automationRowIntent(intent.Kind) {
			return m.nextEffectCmd(), true
		}
		m.reportErr(m.automationManageHint())
	}
	return nil, true
}

// firingLogName names a firing's log tab from its Automation and instant, so
// the same firing always finds the same tab.
func firingLogName(record automations.Record) string {
	id := strings.Trim(nonNameChars.ReplaceAllString(record.AutomationID, "-"), "-")
	return "log-" + id + "-" + record.FiredFor.UTC().Format("20060102T1504")
}

// openFiringLog follows a firing's output in the log tab the Runs view uses.
func (m *Model) openFiringLog(record automations.Record) tea.Cmd {
	if record.LogPath == "" {
		m.reportErr("this firing has no log yet")
		return nil
	}
	return m.openLogTab(firingLogName(record), record.LogPath)
}

func (m *Model) automationLegend() (legendSection, bool) {
	selection, ok := m.rail.Selected()
	if !ok || !isAutomationRow(selection.Kind) {
		return legendSection{}, false
	}
	foldPairs := func(open, closed string) [][2]string {
		action, mark := open, m.glyphs().GroupOpenGlyph
		if m.rail.SelectionCollapsed(selection) {
			action, mark = closed, m.glyphs().GroupClosedGlyph
		}
		pairs := [][2]string{{m.listGlyph(keybind.Open), action}}
		if !m.prefs.mouseDisabled {
			pairs = append(pairs, [2]string{"click " + mark, action})
		}
		return pairs
	}
	switch selection.Kind {
	case uirail.AutomationsRow:
		pairs := append(foldPairs("fold", "unfold"), [2]string{m.listGlyph(keybind.NewAutomation), "new Automation"},
			[2]string{m.listGlyph(keybind.OverviewWindow), "window " + m.workspace.overview.window.Next().Label()},
			[2]string{m.listGlyph(keybind.OverviewUp, keybind.OverviewDown), "scroll"})
		return legendSection{Title: "Automations", Pairs: pairs}, true
	case uirail.AutomationRow:
		pairs := foldPairs("hide firings", "show firings")
		if entry, ok := m.selectedAutomation(); ok {
			if _, fired := entry.Last(); fired {
				pairs = append(pairs, [2]string{m.listGlyph(keybind.Attach), "open last log"})
			}
			toggle := "disable"
			if !entry.Enabled {
				toggle = "enable"
			}
			pairs = append(pairs, [2]string{m.listGlyph(keybind.Rename), "edit"}, [2]string{m.listGlyph(keybind.EnableToggle), toggle},
				[2]string{m.listGlyph(keybind.RunNow), "run now"}, [2]string{m.listGlyph(keybind.Delete), "delete"})
		}
		return legendSection{Title: "Automation", Pairs: pairs}, true
	}
	pairs := [][2]string{{m.listGlyph(keybind.Open, keybind.Attach), "open log"}}
	if !m.prefs.mouseDisabled {
		pairs = append(pairs, [2]string{"double click", "open log"})
	}
	if record, ok := m.selectedFiring(); ok && record.Finished() {
		pairs = append(pairs, [2]string{m.listGlyph(keybind.DebugSession), "debug session"})
	}
	return legendSection{Title: "Firing", Pairs: pairs}, true
}

// automationContentLines fills the content column for the Automations
// section: an Automation's facts and recent firings, or one firing's.
func (m *Model) automationContentLines(width int) ([]contentLine, bool) {
	selection, ok := m.rail.Selected()
	if !ok || !isAutomationRow(selection.Kind) {
		return nil, false
	}
	now := time.Now()
	gutter := "  "
	inner := max(width-len(gutter)*2, 10)
	var lines []contentLine
	wrap := func(text string) {
		for _, line := range uitheme.SplitLines(ansi.Wrap(text, inner, "")) {
			lines = append(lines, contentLine{Text: gutter + line})
		}
	}
	add := func(label, value string) {
		if value != "" {
			wrap(uitheme.LabelStyle().Render(label) + " " + uitheme.ValueStyle().Render(value))
		}
	}
	if held := runs.HeldLine(m.workspace.runs.host, now); held != "" {
		wrap(uitheme.WarnStyle().Render(held))
		lines = append(lines, contentLine{})
	}
	switch selection.Kind {
	case uirail.FiringRow:
		entry, _ := m.selectedAutomation()
		record, ok := m.selectedFiring()
		if !ok {
			return lines, true
		}
		add("Automation", entry.Name)
		add("Fired for", firingClock(record.FiredFor))
		add("Status", record.Status)
		if record.Result != nil {
			add("Outcome", record.Result.Outcome)
			add("Summary", record.Result.Summary)
		}
		if !record.StartedAt.IsZero() {
			add("Started", firingClock(record.StartedAt))
		}
		add("Duration", firingDuration(record, now))
		add("Log", record.LogPath)
		add("Output", record.OutputPath)
		return lines, true
	}
	entry, ok := m.selectedAutomation()
	if !ok {
		return lines, true
	}
	add("Automation", entry.Name)
	if entry.Err != nil {
		add("Unreadable", entry.Err.Error())
		return lines, true
	}
	state := "enabled"
	if !entry.Enabled {
		state = "disabled"
	}
	add("State", state)
	add("Mode", entry.Config.Mode)
	add("Kind", entry.Config.Kind)
	add("Schedule", entry.Config.Cron+" ("+entry.Config.Timezone+")")
	if next, ok := entry.Next(now); ok {
		add("Next", firingClock(next))
	}
	add("Directory", entry.Config.RepoPath)
	add("Tool", entry.Config.Tool)
	add("Entry", entry.Config.Entry)
	lines = append(lines, contentLine{})
	if len(entry.Firings) == 0 {
		wrap(uitheme.LabelStyle().Render("Never fired"))
		return lines, true
	}
	lines = append(lines, contentLine{Text: gutter + uitheme.LabelStyle().Render("Recent firings, newest first")})
	for _, record := range entry.Firings {
		add(firingClock(record.FiredFor), record.Outcome()+" "+firingDuration(record, now))
	}
	return lines, true
}

// firingDuration is how long a firing took, or has run so far.
func firingDuration(record automations.Record, now time.Time) string {
	took, ok := record.Duration(now)
	if !ok {
		return "not started"
	}
	text := took.Round(time.Second).String()
	if !record.Finished() {
		return text + " so far"
	}
	return text
}
