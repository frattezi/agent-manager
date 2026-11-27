package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/keybind"
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	"github.com/YoanWai/agent-manager/internal/ui/runs"
	uianswer "github.com/YoanWai/agent-manager/internal/ui/runs/answer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func runDetail(m *Model) string {
	lines, _ := m.runContentLines(80)
	var text []string
	for _, line := range lines {
		text = append(text, ansi.Strip(line.Text))
	}
	return strings.Join(text, "\n")
}

// answerKeys types text into the open field, enter between lines, then
// presses ctrl+s and lands the save it asked for.
func answerKeys(t *testing.T, m *Model, text string) {
	t.Helper()
	for i, line := range strings.Split(text, "\n") {
		if i > 0 {
			m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
		}
		if line != "" {
			m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(line)})
		}
	}
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd != nil {
		m.applyTestMsg(t, cmd())
	}
}

func readAnswer(t *testing.T, dir, id string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(runs.AnswerPath(dir, id))
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]any
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestPromptKeyAnswersANeedsHumanRunAndWritesTheFile(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	dir := t.TempDir()
	m.applyTestMsg(t, runsLoadedMsg{dir: dir, list: testRuns()})
	selectRun(t, m, "r1")
	if legend, _ := m.runLegend(); !slices.Contains(legend.Pairs, [2]string{m.listGlyph(keybind.Prompt), "answer"}) {
		t.Fatalf("legend = %+v; want the answer key", legend)
	}
	if detail := runDetail(m); !strings.Contains(detail, "Which table owns the column?") || !strings.Contains(detail, "answers the question") {
		t.Fatalf("detail lacks the question or the answer hint:\n%s", detail)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	if _, open := surfaceOf[*uianswer.Surface](m); !open {
		t.Fatal("space on a needs-human Run did not open the answer field")
	}
	if footer := ansi.Strip(m.viewFooter()); !strings.Contains(footer, "ctrl+s") || !strings.Contains(footer, "esc") {
		t.Fatalf("footer = %q; want the field's keys", footer)
	}
	if !strings.Contains(runDetail(m), "Your answer") {
		t.Fatal("the field is not drawn under the question")
	}
	answerKeys(t, m, "orders\nnot users")
	if _, open := surfaceOf[*uianswer.Surface](m); open {
		t.Fatal("the field stayed open after a save")
	}
	file := readAnswer(t, dir, "r1")
	if file["version"] != float64(1) || file["answer"] != "orders\nnot users" || len(file) != 3 {
		t.Fatalf("answer file = %v", file)
	}
	if _, err := time.Parse(time.RFC3339, file["answeredAt"].(string)); err != nil {
		t.Fatalf("answeredAt = %v: %v", file["answeredAt"], err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("dir holds %d files, want only the answer", len(entries))
	}
	if detail := runDetail(m); !strings.Contains(detail, "waiting for the next tick") || !strings.Contains(detail, "not users") {
		t.Fatalf("detail does not show the waiting answer:\n%s", detail)
	}
	if legend, _ := m.runLegend(); !slices.Contains(legend.Pairs, [2]string{m.listGlyph(keybind.Prompt), "edit answer"}) {
		t.Fatalf("legend = %+v; want edit answer", legend)
	}
}

func TestASecondAnswerReopensWithTheFirstAndOverwritesIt(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	dir := t.TempDir()
	if err := runs.WriteAnswer(dir, "r1", "first", time.Now()); err != nil {
		t.Fatal(err)
	}
	m.applyTestMsg(t, runsLoadCmdFor(dir, testRuns()))
	selectRun(t, m, "r1")
	if detail := runDetail(m); !strings.Contains(detail, "waiting for the next tick") || !strings.Contains(detail, "first") {
		t.Fatalf("detail does not show the answer on disk:\n%s", detail)
	}
	m.runIntent(uirail.Intent{Kind: uirail.Prompt, Target: m.railSelection()})
	s, open := surfaceOf[*uianswer.Surface](m)
	if !open || s.Value() != "first" {
		t.Fatalf("field open %v with %q; want the previous answer", open, s.Value())
	}
	answerKeys(t, m, ", then second")
	if got := readAnswer(t, dir, "r1")["answer"]; got != "first, then second" {
		t.Fatalf("answer = %q; want the edited text", got)
	}
}

// runsLoadCmdFor is the poll's message for dir, reading its sidecars.
func runsLoadCmdFor(dir string, list []runs.Record) runsLoadedMsg {
	return runsLoadedMsg{dir: dir, list: list, answers: runs.Answers(dir)}
}

func TestAnEmptyOrCancelledAnswerWritesNothing(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	dir := t.TempDir()
	m.applyTestMsg(t, runsLoadedMsg{dir: dir, list: testRuns()})
	selectRun(t, m, "r1")
	intent := uirail.Intent{Kind: uirail.Prompt, Target: m.railSelection()}
	m.runIntent(intent)
	answerKeys(t, m, "   \n  ")
	if _, open := surfaceOf[*uianswer.Surface](m); !open || !strings.Contains(m.errBar.text, "empty answer") {
		t.Fatalf("open %v, status %q; want the field kept and a refusal", open, m.errBar.text)
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if _, open := surfaceOf[*uianswer.Surface](m); open {
		t.Fatal("esc did not close the field")
	}
	m.runIntent(intent)
	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("draft")})
	m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("a refused or cancelled answer wrote %v", entries)
	}
}

func TestOnlyANeedsHumanRunOffersTheAnswerField(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	dir := t.TempDir()
	list := append(testRuns(),
		runs.Record{Version: 1, ID: "r3", TicketRef: "jira:ABC-3", Status: runs.StatusFailed, LogPath: "/tmp/r3.log", UpdatedAt: time.Now()},
		runs.Record{Version: 1, ID: "r4", TicketRef: "jira:ABC-4", Status: runs.StatusRunning, LogPath: "/tmp/r4.log", UpdatedAt: time.Now()},
	)
	m.applyTestMsg(t, runsLoadedMsg{dir: dir, list: list})
	for _, id := range []string{"r2", "r3", "r4"} {
		selectRun(t, m, id)
		if legend, _ := m.runLegend(); slices.ContainsFunc(legend.Pairs, func(pair [2]string) bool { return strings.Contains(pair[1], "answer") }) {
			t.Fatalf("%s legend = %+v", id, legend)
		}
		if _, taken := m.runIntent(uirail.Intent{Kind: uirail.Prompt, Target: m.railSelection()}); taken {
			t.Fatalf("%s took the prompt key", id)
		}
		if _, open := surfaceOf[*uianswer.Surface](m); open {
			t.Fatalf("%s opened the answer field", id)
		}
		if strings.Contains(runDetail(m), "answer") {
			t.Fatalf("%s detail offers an answer:\n%s", id, runDetail(m))
		}
	}
}

func TestAnUnwritableRegistrySurfacesTheErrorAndKeepsTheDraft(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not bind here")
	}
	t.Parallel()
	m := buildModel(t)
	dir := filepath.Join(t.TempDir(), "runs")
	if err := os.Mkdir(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	m.applyTestMsg(t, runsLoadedMsg{dir: dir, list: testRuns()})
	selectRun(t, m, "r1")
	m.runIntent(uirail.Intent{Kind: uirail.Prompt, Target: m.railSelection()})
	answerKeys(t, m, "yes")
	s, open := surfaceOf[*uianswer.Surface](m)
	if !open || s.Value() != "yes" || !strings.Contains(m.errBar.text, "could not save the answer") {
		t.Fatalf("open %v, status %q; want the draft kept and the error shown", open, m.errBar.text)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("the failed write left %v", entries)
	}
}

func TestTheRunMenuOffersAnswerOnANeedsHumanRun(t *testing.T) {
	t.Parallel()
	m := buildModel(t)
	m.applyTestMsg(t, runsLoadedMsg{dir: t.TempDir(), list: testRuns()})
	got, _ := m.railRuns()
	for _, run := range got {
		if want := run.ID == "r1"; run.CanAnswer != want {
			t.Fatalf("%s CanAnswer = %v", run.ID, run.CanAnswer)
		}
	}
}
