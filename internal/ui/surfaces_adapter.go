package ui

import (
	"time"

	uisurfaces "github.com/YoanWai/agent-manager/internal/ui/surfaces"
	uisurfacespersist "github.com/YoanWai/agent-manager/internal/ui/surfaces/persist"
	tea "github.com/charmbracelet/bubbletea"
)

// surfacesState is which sessions are asking the owner something, as the
// last read found them. One read is on the effect lane at a time; the next
// is scheduled when it finishes.
type surfacesState struct {
	inbox uisurfaces.Inbox
	// reported is the failure last put on the status bar, so a read that
	// keeps failing is reported once.
	reported string
}

type surfacesTickMsg struct{}

// surfacesReadNow reads on startup, so a session that asked while the
// manager was closed is marked straight away.
func surfacesReadNow() tea.Msg { return surfacesTickMsg{} }

func (m *Model) routeSurfacesMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	if _, ok := msg.(surfacesTickMsg); ok {
		m.enqueueEffect(uisurfaces.ReadRequest{}, 0, false)
		return routed(m, nil)
	}
	return nil, nil, false
}

// applySurfacesRead marks the asking sessions and schedules the next read
// on the Runs poll cadence. It never opens a surface: one that seized the
// screen would land while the owner is typing somewhere else.
func (m *Model) applySurfacesRead(inbox uisurfaces.Inbox, err error) tea.Cmd {
	report := ""
	if err != nil {
		report = "could not read what sessions are asking: " + err.Error()
	} else {
		m.workspace.surfaces.inbox = inbox
		m.rebuildRows()
	}
	if report != "" && report != m.workspace.surfaces.reported {
		m.reportErr(report)
	}
	m.workspace.surfaces.reported = report
	return tea.Tick(runsPollInterval, func(time.Time) tea.Msg { return surfacesTickMsg{} })
}

func (s effectServices) surfacesServices() uisurfaces.Services {
	return uisurfacespersist.Services(s.store)
}
