package ui

import (
	"strings"
	"time"

	"github.com/YoanWai/agent-manager/internal/store"
	"github.com/YoanWai/agent-manager/internal/ui/automations"
	tea "github.com/charmbracelet/bubbletea"
)

// automationsState is the clock that fires due Automations. One check is on
// the effect lane at a time; the next is scheduled when it finishes.
type automationsState struct {
	// reported is the failure last put on the status bar, so a firing that
	// keeps failing is reported once rather than on every check.
	reported string
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
	report := strings.Join(problems, "; ")
	if report != "" && report != m.workspace.automations.reported {
		m.reportErr(report)
	}
	m.workspace.automations.reported = report
	return tea.Tick(runsPollInterval, func(time.Time) tea.Msg { return automationsTickMsg{} })
}

// automationStore is the store as a check reads it.
type automationStore struct{ store *store.Store }

func (s automationStore) AutomationRows() ([]automations.Row, error) {
	stored, err := s.store.Automations()
	if err != nil {
		return nil, err
	}
	rows := make([]automations.Row, len(stored))
	for i, a := range stored {
		rows[i] = automationRow(a)
	}
	return rows, nil
}

func (s automationStore) AutomationRow(id string) (automations.Row, bool, error) {
	a, ok, err := s.store.Automation(id)
	return automationRow(a), ok, err
}

func (s automationStore) SetLastFired(id string, firedFor time.Time) error {
	return s.store.SetAutomationLastFired(id, firedFor)
}

func automationRow(a store.Automation) automations.Row {
	return automations.Row{ID: a.ID, Document: a.Config, Enabled: a.Enabled, LastFiredFor: a.LastFiredFor, CreatedAt: a.CreatedAt}
}

// automationServices is the store and the Fractory directories a check runs
// against.
func (s effectServices) automationServices() automations.Services {
	return automations.Services{Store: automationStore{s.store}, RequestsDir: automations.RequestsDir(), RecordsDir: automations.RecordsDir()}
}
