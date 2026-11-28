package ui

import (
	"time"

	"github.com/YoanWai/agent-manager/internal/conversation"
	"github.com/YoanWai/agent-manager/internal/store"
	uimarks "github.com/YoanWai/agent-manager/internal/ui/marks"
	tea "github.com/charmbracelet/bubbletea"
)

// marksState is each session's live Session Marks, as the last read found
// them. One read is on the effect lane at a time; the next is scheduled when
// it finishes.
type marksState struct {
	set uimarks.Set
	// reported is the failure last put on the status bar, so a read that
	// keeps failing is reported once.
	reported string
}

type marksTickMsg struct{}

// marksReadNow reads on startup, so a babysit that outlived a manager
// restart is marked straight away.
func marksReadNow() tea.Msg { return marksTickMsg{} }

func (m *Model) routeMarksMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	if _, ok := msg.(marksTickMsg); ok {
		m.enqueueEffect(uimarks.ReadRequest{}, 0, false)
		return routed(m, nil)
	}
	return nil, nil, false
}

func (m *Model) applyMarksRead(set uimarks.Set, err error) tea.Cmd {
	report := ""
	if err != nil {
		report = "could not read session marks: " + err.Error()
	} else {
		m.workspace.marks.set = set
		m.rebuildRows()
	}
	if report != "" && report != m.workspace.marks.reported {
		m.reportErr(report)
	}
	m.workspace.marks.reported = report
	return tea.Tick(runsPollInterval, func(time.Time) tea.Msg { return marksTickMsg{} })
}

func (m *Model) babysitting(sessionID string) bool {
	return m.workspace.marks.set.Has(sessionID, store.MarkBabysit)
}

type marksStore struct{ store *store.Store }

func (s marksStore) SessionMarks() ([]uimarks.Stored, error) {
	stored, err := s.store.SessionMarks()
	if err != nil {
		return nil, err
	}
	marks := make([]uimarks.Stored, len(stored))
	for i, mark := range stored {
		marks[i] = uimarks.Stored{SessionID: mark.SessionID, Name: mark.Name, Pid: mark.Pid}
	}
	return marks, nil
}

func (s effectServices) marksServices() uimarks.Services {
	return uimarks.Services{Store: marksStore{s.store}, Alive: conversation.Alive}
}
