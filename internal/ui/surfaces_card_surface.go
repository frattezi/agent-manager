package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	uisurfaces "github.com/YoanWai/agent-manager/internal/ui/surfaces"
)

// surfacesCardRoot is the card's view of the root.
type surfacesCardRoot struct{ m *Model }

func (r surfacesCardRoot) ChromeRows(hint [][2]string) int { return r.m.cardChromeRows(hint) }

func (r surfacesCardRoot) GlyphMode() presentation.GlyphMode { return r.m.prefs.glyphMode }

func (r surfacesCardRoot) Send(request uisurfaces.AnswerRequest) tea.Cmd {
	r.m.enqueueEffect(request, 0, false)
	return r.m.nextEffectCmd()
}

// openAgentSurface opens the selected session's current surface over the
// list, drawn from the document the last read stored.
func (m *Model) openAgentSurface() {
	sess, ok := m.selected()
	if !ok {
		return
	}
	name := m.displayName(sess)
	stored, ok := m.workspace.surfaces.inbox.Current(sess.ID)
	if !ok {
		m.reportErr(name + " is not asking anything")
		return
	}
	card, err := uisurfaces.Open(surfacesCardRoot{m}, stored, name)
	if err != nil {
		m.reportErr("could not draw what " + name + " asks: " + err.Error())
		return
	}
	m.clearErr()
	m.push(card)
}
