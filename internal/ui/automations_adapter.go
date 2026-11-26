package ui

import (
	"strings"
	"time"

	"github.com/YoanWai/agent-manager/internal/ui/automations"
	uiautomationspersist "github.com/YoanWai/agent-manager/internal/ui/automations/persist"
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	tea "github.com/charmbracelet/bubbletea"
)

// automationsState is the clock that fires due Automations. One check is on
// the effect lane at a time; the next is scheduled when it finishes.
type automationsState struct {
	// reported is the failure last put on the status bar, so a firing that
	// keeps failing is reported once rather than on every check.
	reported string
	// entries are the stored Automations and their firings as the last
	// check read them; shown is how the rail lists them.
	entries []automations.Entry
	shown   []uirail.Automation
	// zone is this machine's timezone, read once at start, which a new
	// Automation is offered.
	zone string
}

type automationsTickMsg struct{}

// automationsCheckNow asks for a check straight away: the one on startup
// fires an instant that passed while the manager was closed.
func automationsCheckNow() tea.Msg { return automationsTickMsg{} }

func (m *Model) routeAutomationsMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	if _, ok := msg.(automationsTickMsg); ok {
		m.enqueueEffect(automations.CheckRequest{Now: time.Now()}, 0, false)
		return routed(m, nil)
	}
	return nil, nil, false
}

// applyAutomationsCheck reports what a check could not do and schedules the
// next one on the Runs poll cadence.
func (m *Model) applyAutomationsCheck(result automations.CheckResult, err error) tea.Cmd {
	var problems []string
	if err != nil {
		problems = append(problems, err.Error())
	}
	for _, failure := range result.Failed {
		problems = append(problems, "Automation "+failure.ID+" did not fire for "+failure.FiredFor.Format(time.RFC3339)+": "+failure.Reason.Error())
	}
	for _, unreadable := range result.Unreadable {
		problems = append(problems, "Automation "+unreadable.ID+" cannot be scheduled: "+unreadable.Reason.Error())
	}
	if err == nil {
		m.showAutomations(result.Rows, result.Records)
	}
	report := strings.Join(problems, "; ")
	if report != "" && report != m.workspace.automations.reported {
		m.reportErr(report)
	}
	m.workspace.automations.reported = report
	return tea.Tick(runsPollInterval, func(time.Time) tea.Msg { return automationsTickMsg{} })
}

// automationServices is the store and the Fractory directories a check runs
// against.
func (s effectServices) automationServices() automations.Services {
	return uiautomationspersist.Services(s.store)
}
