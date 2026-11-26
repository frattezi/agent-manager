package ui

import (
	"reflect"
	"strings"
	"time"

	"github.com/YoanWai/agent-manager/internal/keybind"
	"github.com/YoanWai/agent-manager/internal/store"
	"github.com/YoanWai/agent-manager/internal/ui/automations"
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	"github.com/YoanWai/agent-manager/internal/ui/runs"
	uitheme "github.com/YoanWai/agent-manager/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// automationsState is the stored Automations and Fractory's Automation Run
// records as last read. The rail lists both read-only.
type automationsState struct {
	stored  []store.Automation
	records []automations.Record
	// rail is the section as last handed to the rail, so a poll that
	// changes nothing on screen does not rebuild the rows.
	rail []uirail.Automation
}

type automationsLoadedMsg struct {
	stored  []store.Automation
	records []automations.Record
}

type automationsTickMsg struct{}

// automationsLoadCmd reads the store and the records directory off the
// update path. A records directory that is absent or unreadable lists no
// firings; the next poll tries again.
func (m *Model) automationsLoadCmd() tea.Cmd {
	st := m.services.store
	return func() tea.Msg {
		stored, _ := st.Automations()
		var records []automations.Record
		if dir := automations.RecordsDir(); dir != "" {
			records, _, _ = automations.LoadRecords(dir)
		}
		return automationsLoadedMsg{stored: stored, records: records}
	}
}

func (m *Model) routeAutomationsMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case automationsLoadedMsg:
		m.workspace.automations.stored = msg.stored
		m.workspace.automations.records = msg.records
		if section := m.railAutomations(time.Now()); !reflect.DeepEqual(section, m.workspace.automations.rail) {
			m.workspace.automations.rail = section
			m.rebuildRows()
		}
		return routed(m, tea.Tick(runsPollInterval, func(time.Time) tea.Msg { return automationsTickMsg{} }))
	case automationsTickMsg:
		return routed(m, m.automationsLoadCmd())
	}
	return nil, nil, false
}

// railAutomations is one rail row per stored Automation, each with its
// firings newest first.
func (m *Model) railAutomations(now time.Time) []uirail.Automation {
	if len(m.workspace.automations.stored) == 0 {
		return nil
	}
	byAutomation := automations.ByAutomation(m.workspace.automations.records)
	out := make([]uirail.Automation, len(m.workspace.automations.stored))
	for i, stored := range m.workspace.automations.stored {
		config, _ := automations.Describe(stored.Config)
		row := uirail.Automation{ID: stored.ID, Name: stored.Name, Mode: config.Mode, Enabled: stored.Enabled, Last: uirail.NeverFired}
		if stored.Enabled {
			if next, err := automations.NextFire(config, now); err == nil {
				row.Next = clockNear(next, now)
			}
		}
		records := byAutomation[stored.ID]
		if len(records) > 0 {
			row.Last, row.LastStatus = records[0].Outcome(), records[0].Status
		}
		for _, record := range records {
			row.Firings = append(row.Firings, uirail.Firing{
				ID: record.ID, When: clockNear(record.FiredFor, now), Status: record.Status, Duration: firingDuration(record, now),
			})
		}
		out[i] = row
	}
	return out
}

// clockNear is an instant in the local time zone, as short as now allows:
// 07:00 today, Mon 07:00 within the week, Jan 2 07:00 otherwise.
func clockNear(at, now time.Time) string {
	at, now = at.Local(), now.Local()
	day := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()) }
	switch apart := day(at).Sub(day(now)); {
	case apart == 0:
		return at.Format("15:04")
	case apart > -7*24*time.Hour && apart < 7*24*time.Hour:
		return at.Format("Mon 15:04")
	case at.Year() == now.Year():
		return at.Format("Jan 2 15:04")
	}
	return at.Format("2006-01-02 15:04")
}

func firingDuration(record automations.Record, now time.Time) string {
	took, ok := record.Duration(now)
	if !ok {
		return ""
	}
	return automations.FormatDuration(took)
}

func isAutomationKind(kind uirail.RowKind) bool {
	return kind == uirail.AutomationsRow || kind == uirail.AutomationRow || kind == uirail.FiringRow
}

// selectedAutomation is the Automation the rail's cursor is on, or the one
// whose firing it is on, with that Automation's records newest first.
func (m *Model) selectedAutomation() (store.Automation, []automations.Record, bool) {
	selection, ok := m.rail.Selected()
	if !ok || (selection.Kind != uirail.AutomationRow && selection.Kind != uirail.FiringRow) {
		return store.Automation{}, nil, false
	}
	for _, stored := range m.workspace.automations.stored {
		if stored.ID == selection.Automation {
			return stored, automations.ByAutomation(m.workspace.automations.records)[stored.ID], true
		}
	}
	return store.Automation{}, nil, false
}

// selectedFiring is the firing the rail's cursor is on, or on an
// Automation row its latest.
func (m *Model) selectedFiring() (automations.Record, bool) {
	selection, ok := m.rail.Selected()
	if !ok {
		return automations.Record{}, false
	}
	_, records, ok := m.selectedAutomation()
	if !ok || len(records) == 0 {
		return automations.Record{}, false
	}
	if selection.Kind == uirail.AutomationRow {
		return records[0], true
	}
	for _, record := range records {
		if record.ID == selection.Firing {
			return record, true
		}
	}
	return automations.Record{}, false
}

// automationIntent runs a rail intent aimed at the Automations section and
// reports whether it took it. Enter opens a firing's log; the keys that
// change a session refuse.
func (m *Model) automationIntent(intent uirail.Intent) (tea.Cmd, bool) {
	if !isAutomationKind(intent.Target.Kind) {
		return nil, false
	}
	switch intent.Kind {
	case uirail.Quit, uirail.OpenSettings, uirail.Resize, uirail.OpenMessages, uirail.OpenHelp, uirail.Prompt, uirail.NewSession, uirail.NewGroup, uirail.NewConnection:
		return nil, false
	case uirail.Focus, uirail.Attach:
		if intent.Target.Kind == uirail.AutomationsRow {
			return nil, true
		}
		record, ok := m.selectedFiring()
		if !ok {
			m.reportErr("This Automation has not fired yet")
			return nil, true
		}
		if record.LogPath == "" {
			m.reportErr("This firing has no log yet")
			return nil, true
		}
		stored, _, _ := m.selectedAutomation()
		return m.openLogTab(firingLogName(stored, record), record.LogPath), true
	default:
		m.reportErr("Automations are read-only here: change them with agent-manager automations")
	}
	return nil, true
}

// firingLogName names a firing's log tab from the firing, so the same one
// always finds the same tab: log-<automation name>-<first 4 of the run id>.
func firingLogName(stored store.Automation, record automations.Record) string {
	name := strings.Trim(nonNameChars.ReplaceAllString(stored.Name, "-"), "-")
	return "log-" + name + "-" + record.ID[:min(4, len(record.ID))]
}

func (m *Model) automationLegend() (legendSection, bool) {
	selection, ok := m.rail.Selected()
	if !ok || !isAutomationKind(selection.Kind) {
		return legendSection{}, false
	}
	if selection.Kind == uirail.AutomationsRow {
		fold, mark := "fold", m.glyphs().GroupOpenGlyph
		if m.rail.SelectionCollapsed(selection) {
			fold, mark = "unfold", m.glyphs().GroupClosedGlyph
		}
		pairs := [][2]string{{m.listGlyph(keybind.Open), fold}}
		if !m.prefs.mouseDisabled {
			pairs = append(pairs, [2]string{"click " + mark, fold})
		}
		return legendSection{Title: "Automations", Pairs: pairs}, true
	}
	var pairs [][2]string
	if _, ok := m.selectedFiring(); ok {
		action := "open log"
		if selection.Kind == uirail.AutomationRow {
			action = "open latest log"
		}
		pairs = append(pairs, [2]string{m.listGlyph(keybind.Open, keybind.Attach), action})
		if !m.prefs.mouseDisabled {
			pairs = append(pairs, [2]string{"double click", action})
		}
	}
	title := "Automation"
	if selection.Kind == uirail.FiringRow {
		title = "Firing"
	}
	return legendSection{Title: title, Pairs: pairs}, true
}

// automationContentLines fills the content column for an Automation, its
// recent firings, or one firing, under the Held line Runs show.
func (m *Model) automationContentLines(width int) ([]contentLine, bool) {
	selection, ok := m.rail.Selected()
	if !ok || !isAutomationKind(selection.Kind) {
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
	stored, records, ok := m.selectedAutomation()
	if !ok {
		return lines, len(lines) > 0
	}
	config, err := automations.Describe(stored.Config)
	if selection.Kind == uirail.FiringRow {
		record, ok := m.selectedFiring()
		if !ok {
			return lines, len(lines) > 0
		}
		add("Automation", stored.Name)
		add("Fired for", clockNear(record.FiredFor, now))
		add("Status", record.Status)
		if outcome := record.Outcome(); outcome != record.Status {
			add("Outcome", outcome)
		}
		if !record.StartedAt.IsZero() {
			add("Started", clockNear(record.StartedAt, now))
		}
		add("Took", firingDuration(record, now))
		add("Log", record.LogPath)
		add("Worktree", record.WorktreePath)
		if record.Result != nil && record.Result.Summary != "" {
			lines = append(lines, contentLine{})
			wrap(record.Result.Summary)
		}
		return lines, true
	}
	add("Automation", stored.Name)
	if err != nil {
		add("Configuration", "unreadable: "+err.Error())
	} else {
		add("Mode", config.Mode)
		add("Kind", config.Kind)
		add("Tool", config.Tool)
		add("Directory", config.RepoPath)
		add("Entry", config.Entry)
		add("Schedule", strings.TrimSpace(config.Cron+" "+config.Timezone))
	}
	switch next, nextErr := automations.NextFire(config, now); {
	case !stored.Enabled:
		add("Next", "disabled")
	case err == nil && nextErr != nil:
		add("Next", "schedule unreadable: "+nextErr.Error())
	case err == nil:
		add("Next", clockNear(next, now))
	}
	lines = append(lines, contentLine{})
	if len(records) == 0 {
		wrap(uitheme.LabelStyle().Render("Never fired"))
		return lines, true
	}
	wrap(uitheme.LabelStyle().Render("Firings, newest first"))
	for _, record := range records {
		wrap(uitheme.ValueStyle().Render(strings.Join(nonEmpty(clockNear(record.FiredFor, now), record.Outcome(), firingDuration(record, now)), "  ")))
	}
	return lines, true
}

func nonEmpty(values ...string) []string {
	out := values[:0]
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
