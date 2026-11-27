package surfaces

import (
	"strings"

	"github.com/YoanWai/agent-manager/internal/ui/a2ui"
	uiframe "github.com/YoanWai/agent-manager/internal/ui/frame"
	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	"github.com/YoanWai/agent-manager/internal/ui/surface"
	uitheme "github.com/YoanWai/agent-manager/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// Root is what the card reads from the root beyond surface.Host.
type Root interface {
	// ChromeRows is the rows a card spends around its body with hint on
	// its foot, status block included.
	ChromeRows(hint [][2]string) int
	GlyphMode() presentation.GlyphMode
	// Send hands the answer to the effect lane; the card stays open until
	// the root reports it queued, so a refusal keeps what was chosen.
	Send(AnswerRequest) tea.Cmd
}

// Card draws a session's current surface over the list as a form the owner
// moves through, steps and types into. Only the send key sends what it
// holds: closing it leaves the surface unanswered, and the session still
// asking.
type Card struct {
	root      Root
	sessionID string
	surfaceID string
	name      string
	doc       a2ui.Document
	// published is the round as the agent wrote it, which the answer is
	// read against to tell a recommendation kept from one overridden.
	published a2ui.Document
	// sending holds a second send while the first is on the lane.
	sending bool
	fields  []*a2ui.Node
	focus   a2ui.Focus
	offset  int
}

// Open parses the stored document into a card titled for the session it
// came from. A document this build refuses does not open.
func Open(root Root, stored Stored, name string) (*Card, error) {
	doc, err := a2ui.Parse([]byte(stored.Document))
	if err != nil {
		return nil, err
	}
	published, err := a2ui.Parse([]byte(stored.Document))
	if err != nil {
		return nil, err
	}
	c := &Card{root: root, sessionID: stored.SessionID, surfaceID: stored.ID, name: name, doc: doc, published: published}
	c.fields = a2ui.Fields(&c.doc)
	if len(c.fields) > 0 {
		c.focusField(1)
	}
	return c, nil
}

func (c *Card) SessionID() string { return c.sessionID }

func (c *Card) SurfaceID() string { return c.surfaceID }

// Focus is the field the card is on and the choice under its cursor.
func (c *Card) Focus() a2ui.Focus { return c.focus }

// Document is the round as the owner has answered it so far.
func (c *Card) Document() a2ui.Document { return c.doc }

func (c *Card) field() *a2ui.Node {
	if c.focus.Field < 1 || c.focus.Field > len(c.fields) {
		return nil
	}
	return c.fields[c.focus.Field-1]
}

// focusField moves to a field, its cursor on the choice already chosen.
func (c *Card) focusField(n int) {
	c.focus = a2ui.Focus{Field: n, Option: 1}
	if f := c.field(); f != nil && f.Type == "ChoicePicker" {
		for i, option := range f.Options {
			if option.Selected {
				c.focus.Option = i + 1
				break
			}
		}
	}
}

func (c *Card) move(delta int) {
	if len(c.fields) == 0 {
		return
	}
	c.focusField(min(max(c.focus.Field+delta, 1), len(c.fields)))
}

// step moves a choice: a single choice moves what is chosen, a multiple
// choice moves the cursor that space toggles under.
func (c *Card) step(delta int) {
	f := c.field()
	if f == nil || f.Type != "ChoicePicker" || len(f.Options) == 0 {
		return
	}
	c.focus.Option = min(max(c.focus.Option+delta, 1), len(f.Options))
	if !f.Multiple {
		c.choose(f, c.focus.Option)
	}
}

func (c *Card) choose(f *a2ui.Node, option int) {
	if option < 1 || option > len(f.Options) {
		return
	}
	if f.Multiple {
		f.Options[option-1].Selected = !f.Options[option-1].Selected
		return
	}
	for i := range f.Options {
		f.Options[i].Selected = i == option-1
	}
}

func (c *Card) toggle() {
	f := c.field()
	if f == nil {
		return
	}
	switch f.Type {
	case "CheckBox":
		f.Checked = !f.Checked
	case "ChoicePicker":
		c.choose(f, c.focus.Option)
	}
}

// typeInto edits a focused text field and reports whether the key was one
// it takes.
func (c *Card) typeInto(msg tea.KeyMsg) bool {
	f := c.field()
	if f == nil || f.Type != "TextField" {
		return false
	}
	switch msg.Type {
	case tea.KeyRunes, tea.KeySpace:
		f.Value += string(msg.Runes)
	case tea.KeyBackspace:
		if runes := []rune(f.Value); len(runes) > 0 {
			f.Value = string(runes[:len(runes)-1])
		}
	case tea.KeyCtrlU:
		f.Value = ""
	default:
		return false
	}
	return true
}

// send queues the round as answered. Only the owner's key or click calls
// it: nothing on a timer, a poll or a close does.
func (c *Card) send() tea.Cmd {
	if c.sending {
		return nil
	}
	c.sending = true
	return c.root.Send(AnswerRequest{
		SessionID:   c.sessionID,
		SurfaceID:   c.surfaceID,
		SurfaceName: c.doc.SurfaceID,
		Answer:      AnswerText(c.published, c.doc),
	})
}

// Refused takes the card back from sending after the answer was not
// queued, so the owner can send again.
func (c *Card) Refused() { c.sending = false }

func (c *Card) Key(h surface.Host, msg tea.KeyMsg) (bool, tea.Cmd) {
	switch msg.String() {
	case "esc":
		h.Pop()
		return true, nil
	case "enter":
		return true, c.send()
	case "tab", "down":
		c.move(1)
	case "shift+tab", "up":
		c.move(-1)
	case "left":
		c.step(-1)
	case "right":
		c.step(1)
	default:
		if !c.typeInto(msg) && msg.Type == tea.KeySpace {
			c.toggle()
		}
	}
	c.follow(h)
	return true, nil
}

// Click focuses the field under the pointer: a choice is chosen, or
// toggled where several may be, and a check box flips.
func (c *Card) Click(h surface.Host, line int) tea.Cmd {
	_, inner, rows := c.size(h)
	_, targets := a2ui.RenderForm(c.doc, inner, c.focus)
	at := clampOffset(c.offset, len(targets), rows) + line
	if line < 0 || at >= len(targets) || targets[at].Field == 0 {
		return nil
	}
	target := targets[at]
	c.focusField(target.Field)
	f := c.field()
	switch {
	case f.Type == "ChoicePicker" && target.Option > 0:
		c.focus.Option = target.Option
		c.choose(f, target.Option)
	case f.Type == "CheckBox":
		f.Checked = !f.Checked
	}
	return nil
}

// Wheel scrolls a round taller than the card.
func (c *Card) Wheel(h surface.Host, arrow tea.KeyMsg) tea.Cmd {
	_, inner, rows := c.size(h)
	lines, _ := a2ui.RenderForm(c.doc, inner, c.focus)
	switch arrow.Type {
	case tea.KeyUp:
		c.offset--
	case tea.KeyDown:
		c.offset++
	}
	c.offset = clampOffset(c.offset, len(lines), rows)
	return nil
}

// follow scrolls the window just far enough to show the focused field.
func (c *Card) follow(h surface.Host) {
	_, inner, rows := c.size(h)
	_, targets := a2ui.RenderForm(c.doc, inner, c.focus)
	first, last := -1, -1
	for i, t := range targets {
		if t.Field == c.focus.Field && c.focus.Field != 0 {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first >= 0 {
		c.offset = presentation.Follow(c.offset, last, len(targets), rows)
		c.offset = presentation.Follow(c.offset, first, len(targets), rows)
	}
	c.offset = clampOffset(c.offset, len(targets), rows)
}

func clampOffset(offset, count, rows int) int {
	return min(max(offset, 0), max(count-rows, 0))
}

// size is the card's width, the width the round is drawn at, and the rows
// its body shows.
func (c *Card) size(h surface.Host) (width, inner, rows int) {
	cols, height := h.Size()
	width = uiframe.CardWidthFor(cols)
	return width, uiframe.CardInnerWidth(width), max(height-c.root.ChromeRows(c.hints()), 1)
}

func (c *Card) hints() [][2]string {
	hints := [][2]string{{"tab", "next question"}, {"shift+tab", "previous"}}
	if f := c.field(); f != nil {
		switch f.Type {
		case "ChoicePicker":
			hints = append(hints, [2]string{"←/→", "step choice"})
			if f.Multiple {
				hints = append(hints, [2]string{"space", "toggle"})
			}
		case "CheckBox":
			hints = append(hints, [2]string{"space", "toggle"})
		case "TextField":
			hints = append(hints, [2]string{"type", "answer"}, [2]string{"⌫", "delete"})
		}
	}
	return append(hints, [2]string{"↵", "send"}, [2]string{"esc", "close, still unanswered"})
}

func (c *Card) View(h surface.Host, _ string) string {
	width, inner, rows := c.size(h)
	lines, _ := a2ui.RenderForm(c.doc, inner, c.focus)
	window := presentation.Window(lines, rows, clampOffset(c.offset, len(lines), rows), uitheme.SubtleStyle(), c.root.GlyphMode())
	return h.CardSized(width, presentation.EscapeControlsInline(c.name)+" is asking", strings.Join(window, "\n"), c.hints())
}

// Footer is empty: the card names its own keys on its foot.
func (c *Card) Footer() [][2]string { return nil }

func (c *Card) Owner() surface.Owner { return surface.Manager }

// Dismissible: a round holds what the owner typed, so a stray click keeps
// it; esc closes the card.
func (c *Card) Dismissible() bool { return false }

func (c *Card) Leave(surface.Host) {}
