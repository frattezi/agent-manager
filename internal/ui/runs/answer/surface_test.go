package answer

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

type answerHost struct{ popped int }

func (h *answerHost) CardSized(int, string, string, [][2]string) string { return "" }
func (h *answerHost) Size() (int, int)                                  { return 100, 40 }
func (h *answerHost) Pop()                                              { h.popped++ }

type answerStub struct {
	errs  []string
	saved []string
}

func (r *answerStub) ReportErr(text string) { r.errs = append(r.errs, text) }

func (r *answerStub) SaveAnswer(runID, answer string) tea.Cmd {
	r.saved = append(r.saved, runID+"="+answer)
	return nil
}

func typeInto(s *Surface, h *answerHost, text string) {
	for _, line := range strings.SplitAfter(text, "\n") {
		body := strings.TrimSuffix(line, "\n")
		if body != "" {
			s.Key(h, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(body)})
		}
		if body != line {
			s.Key(h, tea.KeyMsg{Type: tea.KeyEnter})
		}
	}
}

func TestAnswerFieldSavesWhatWasTypedNewlinesIncluded(t *testing.T) {
	root, h := &answerStub{}, &answerHost{}
	s := New(root, "r1", "")
	typeInto(s, h, "orders\nnot users")
	if handled, _ := s.Key(h, tea.KeyMsg{Type: tea.KeyCtrlS}); !handled {
		t.Fatal("ctrl+s was not handled")
	}
	if len(root.saved) != 1 || root.saved[0] != "r1=orders\nnot users" {
		t.Fatalf("saved = %q", root.saved)
	}
	s.Key(h, tea.KeyMsg{Type: tea.KeyCtrlS})
	if len(root.saved) != 1 {
		t.Fatal("a second ctrl+s while saving saved again")
	}
	if got := s.Footer(); len(got) != 1 || got[0][0] != "wait" {
		t.Fatalf("footer while saving = %v", got)
	}
	s.Settled()
	if got := s.Footer(); len(got) != 3 || got[0] != [2]string{"ctrl+s", "save"} {
		t.Fatalf("footer = %v", got)
	}
	if h.popped != 0 {
		t.Fatal("the field closed itself before the save landed")
	}
}

func TestAnswerFieldRefusesAnEmptyAnswer(t *testing.T) {
	root, h := &answerStub{}, &answerHost{}
	s := New(root, "r1", "")
	typeInto(s, h, "  \n ")
	s.Key(h, tea.KeyMsg{Type: tea.KeyCtrlS})
	if len(root.saved) != 0 || len(root.errs) != 1 || !strings.Contains(root.errs[0], "empty answer") {
		t.Fatalf("saved %q, errs %q; want a refusal", root.saved, root.errs)
	}
}

func TestAnswerFieldEscCancelsWithoutSaving(t *testing.T) {
	root, h := &answerStub{}, &answerHost{}
	s := New(root, "r1", "")
	typeInto(s, h, "draft")
	s.Key(h, tea.KeyMsg{Type: tea.KeyEsc})
	if h.popped != 1 || len(root.saved) != 0 {
		t.Fatalf("popped %d, saved %q", h.popped, root.saved)
	}
	if s.Dismissible() {
		t.Fatal("a draft field is dismissible")
	}
}

func TestAnswerFieldReopensWithThePreviousAnswer(t *testing.T) {
	root, h := &answerStub{}, &answerHost{}
	s := New(root, "r1", "first\nline")
	if s.Value() != "first\nline" {
		t.Fatalf("value = %q", s.Value())
	}
	if view := s.FieldView(40); !strings.Contains(view, "first") || !strings.Contains(view, "line") {
		t.Fatalf("view lacks the previous answer:\n%s", view)
	}
	typeInto(s, h, " more")
	s.Key(h, tea.KeyMsg{Type: tea.KeyCtrlS})
	if len(root.saved) != 1 || root.saved[0] != "r1=first\nline more" {
		t.Fatalf("saved = %q", root.saved)
	}
}
