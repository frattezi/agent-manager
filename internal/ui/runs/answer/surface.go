// Package answer is the field the owner types an answer to a needs-human
// Run in, under its question.
package answer

import (
	"strings"

	uiframe "github.com/YoanWai/agent-manager/internal/ui/frame"
	"github.com/YoanWai/agent-manager/internal/ui/surface"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// rows is how many rows the field paints; a longer answer scrolls.
const rows = 6

// Surface is the owner's answer to a needs-human Run, typed under its
// question in the content column. It paints no card: the column draws the
// field, and the list holds still under it.
type Surface struct {
	runID  string
	input  textarea.Model
	saving bool
	root   Root
}

// Root is what the field reaches on the root beyond surface.Host.
type Root interface {
	ReportErr(text string)
	// SaveAnswer writes the answer off the update path; its result lands
	// back on the field through Saved.
	SaveAnswer(runID, answer string) tea.Cmd
}

// New opens the field for the Run, holding previous: the answer
// already waiting for the next tick, which a save replaces.
func New(root Root, runID, previous string) *Surface {
	input := textarea.New()
	input.CharLimit = 64 * 1024
	input.MaxHeight = 0
	input.ShowLineNumbers = false
	input.Placeholder = "your answer to the Run's question"
	input.FocusedStyle.CursorLine = lipgloss.NewStyle()
	input.SetHeight(rows)
	input.SetValue(previous)
	input.Focus()
	return &Surface{runID: runID, input: input, root: root}
}

// RunID is the Run the answer is for.
func (s *Surface) RunID() string { return s.runID }

// Value is the answer as typed.
func (s *Surface) Value() string { return s.input.Value() }

// Settled ends a save in flight; a failed one leaves the draft to retry.
func (s *Surface) Settled() { s.saving = false }

func (s *Surface) Key(h surface.Host, msg tea.KeyMsg) (bool, tea.Cmd) {
	if s.saving {
		return true, nil
	}
	switch msg.String() {
	case "esc":
		h.Pop()
		return true, nil
	case "ctrl+s":
		if strings.TrimSpace(s.input.Value()) == "" {
			s.root.ReportErr("an empty answer is not saved: type one, or esc to cancel")
			return true, nil
		}
		s.saving = true
		return true, s.root.SaveAnswer(s.runID, s.input.Value())
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return true, cmd
}

// FieldView paints the field width cells wide, with the caret marked for the
// IME anchor.
func (s *Surface) FieldView(width int) string {
	s.input.SetWidth(max(width, 10))
	return uiframe.TextAreaView(s.input)
}

func (s *Surface) Click(surface.Host, int) tea.Cmd { return nil }

func (s *Surface) Wheel(surface.Host, tea.KeyMsg) tea.Cmd { return nil }

// Mouse holds the screen still while an answer is typed.
func (s *Surface) Mouse(surface.Host, tea.MouseMsg) tea.Cmd { return nil }

// View leaves the frame as it is: the content column draws the field.
func (s *Surface) View(_ surface.Host, below string) string { return below }

func (s *Surface) Footer() [][2]string {
	if s.saving {
		return [][2]string{{"wait", "saving the answer"}}
	}
	return [][2]string{{"ctrl+s", "save"}, {"↵", "new line"}, {"esc", "cancel"}}
}

func (s *Surface) Owner() surface.Owner { return surface.Manager }

// Dismissible: the answer is a draft, so a stray click or ctrl+c keeps it;
// esc cancels.
func (s *Surface) Dismissible() bool { return false }

func (s *Surface) Leave(surface.Host) {}
