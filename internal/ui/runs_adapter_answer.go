package ui

import (
	"maps"
	"time"

	"github.com/YoanWai/agent-manager/internal/keybind"
	"github.com/YoanWai/agent-manager/internal/ui/runs"
	uianswer "github.com/YoanWai/agent-manager/internal/ui/runs/answer"
	uitheme "github.com/YoanWai/agent-manager/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

const runAnswerSavedNotice = "Answer saved; the factory will pick it up on the next tick"

// runsAnswerSavedMsg is the result of writing an answer file.
type runsAnswerSavedMsg struct {
	runID  string
	answer string
	err    error
}

// answerRoot is the Run answer field's view of the root.
type answerRoot struct{ m *Model }

func (r answerRoot) ReportErr(text string) { r.m.reportErr(text) }

// SaveAnswer writes the answer file off the update path, into the registry
// the Runs were read from.
func (r answerRoot) SaveAnswer(runID, answer string) tea.Cmd {
	dir := r.m.workspace.runs.dir
	r.m.clearErr()
	return func() tea.Msg {
		return runsAnswerSavedMsg{runID: runID, answer: answer, err: runs.WriteAnswer(dir, runID, answer, time.Now())}
	}
}

// openAnswer opens the answer field under the Run's question, holding the
// answer already waiting for the next tick.
func (m *Model) openAnswer(run runs.Record) {
	if m.workspace.runs.dir == "" {
		m.reportErr("no Run registry to write an answer to")
		return
	}
	m.clearErr()
	m.push(uianswer.New(answerRoot{m}, run.ID, m.workspace.runs.answers[run.ID]))
}

// answerSaved settles a save on the field that asked for it: a failure keeps
// the draft open and says why, a success closes it and shows the answer as
// waiting.
func (m *Model) answerSaved(msg runsAnswerSavedMsg) {
	s, open := surfaceOf[*uianswer.Surface](m)
	asked := open && s.RunID() == msg.runID
	if asked {
		s.Settled()
	}
	if msg.err != nil {
		m.reportErr("could not save the answer: " + msg.err.Error())
		return
	}
	if asked {
		m.Pop()
	}
	answers := maps.Clone(m.workspace.runs.answers)
	if answers == nil {
		answers = map[string]string{}
	}
	answers[msg.runID] = msg.answer
	m.workspace.runs.answers = answers
	m.reportDone(runAnswerSavedNotice)
	m.rebuildRows()
}

// answerAction is what the prompt key does on a needs-human Run.
func (m *Model) answerAction(run runs.Record) string {
	if m.workspace.runs.answers[run.ID] != "" {
		return "edit answer"
	}
	return "answer"
}

// answerLines is the answer part of a needs-human Run's column: the open
// field, or the answer waiting for the next tick, or how to give one.
func (m *Model) answerLines(run runs.Record, gutter string, inner int) []contentLine {
	lines := []contentLine{{}}
	if s, ok := surfaceOf[*uianswer.Surface](m); ok && s.RunID() == run.ID {
		lines = append(lines, contentLine{Text: gutter + uitheme.LabelStyle().Render("Your answer")})
		for _, line := range uitheme.SplitLines(s.FieldView(inner)) {
			lines = append(lines, contentLine{Text: gutter + line})
		}
		return lines
	}
	if answer := m.workspace.runs.answers[run.ID]; answer != "" {
		lines = append(lines, contentLine{Text: gutter + uitheme.WarnStyle().Render("Answered, waiting for the next tick")})
		for _, line := range uitheme.SplitLines(ansi.Wrap(answer, inner, "")) {
			lines = append(lines, contentLine{Text: gutter + uitheme.ValueStyle().Render(line)})
		}
		if hint := m.listKeyHint(keybind.Prompt, "%s edits the answer"); hint != "" {
			lines = append(lines, contentLine{Text: gutter + uitheme.LabelStyle().Render(hint)})
		}
		return lines
	}
	if hint := m.listKeyHint(keybind.Prompt, "%s answers the question"); hint != "" {
		return append(lines, contentLine{Text: gutter + uitheme.LabelStyle().Render(hint)})
	}
	return nil
}
