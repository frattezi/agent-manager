package bots

import (
	uidialogs "github.com/YoanWai/agent-manager/internal/ui/dialogs"
	"github.com/YoanWai/agent-manager/internal/ui/surface"
	tea "github.com/charmbracelet/bubbletea"
)

// Surface is the Bots dialog as a card on the root's stack. The root stores
// what it builds, on its effect lane.
type Surface struct {
	dialog Dialog
	root   Root
}

// Root is what the card reaches on the root beyond surface.Host.
type Root interface {
	Look() Look
	CardWidth(cols int) int
	// Save queues the Profile the dialog gen built: a new one when editing
	// is empty, else a replacement for the one it names.
	Save(gen uint64, editing string, p Profile) tea.Cmd
}

func NewSurface(root Root, dialog Dialog) *Surface {
	return &Surface{dialog: dialog, root: root}
}

func (s *Surface) Dialog() *Dialog { return &s.dialog }

func (s *Surface) Key(h surface.Host, msg tea.KeyMsg) (bool, tea.Cmd) {
	// A pasted run is text: without this a pasted "tab" is the focus key.
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 1 && !msg.Alt {
		msg.Paste = true
	}
	outcome := s.dialog.Key(msg)
	switch outcome.Action {
	case Cancel:
		h.Pop()
		return true, nil
	case Save:
		return true, s.root.Save(outcome.Gen, outcome.Editing, outcome.Profile)
	}
	return true, outcome.Cmd
}

func (s *Surface) Click(_ surface.Host, line int) tea.Cmd { return s.dialog.Click(line) }

// Wheel: the fields fit the card, there is nothing to scroll.
func (s *Surface) Wheel(surface.Host, tea.KeyMsg) tea.Cmd { return nil }

func (s *Surface) View(h surface.Host, _ string) string {
	cols, _ := h.Size()
	width := s.root.CardWidth(cols)
	s.dialog.SyncWidths(uidialogs.FieldWidth(width))
	look := s.root.Look()
	look.Width = uidialogs.CardInnerWidth(width)
	card := s.dialog.View(look)
	return h.CardSized(width, card.Title, card.Body, card.Hints)
}

// Footer is empty: the card paints its own hints.
func (s *Surface) Footer() [][2]string { return nil }

func (s *Surface) Owner() surface.Owner { return surface.Manager }

// Dismissible: the fields hold a draft, so a stray click keeps it.
func (s *Surface) Dismissible() bool { return false }

func (s *Surface) Leave(surface.Host) {}
