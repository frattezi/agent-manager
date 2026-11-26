package form

import (
	"errors"
	"strings"
	"testing"

	"github.com/YoanWai/agent-manager/internal/ui/automations"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func look() Look {
	return Look{
		Field: func(label, value string, focused bool) string { return label + " " + value + "\n" },
		Input: func(in textinput.Model) string { return in.Value() },
		Width: 60, Value: lipgloss.NewStyle(), Subtle: lipgloss.NewStyle(), Muted: lipgloss.NewStyle(), Err: lipgloss.NewStyle(),
	}
}

func TestANewDialogStartsFromTheSeed(t *testing.T) {
	d := New(Seed{Dir: "/src/repo", Timezone: "Europe/Lisbon", Tools: []string{"claude", "codex"}}, 1)
	d.inputs[NameField].SetValue("weekly")
	d.inputs[CronField].SetValue("0 7 * * 1")
	d.inputs[EntryField].SetValue("prompts/weekly.md")
	out := d.Key(tea.KeyMsg{Type: tea.KeyEnter})
	c := out.Config
	if out.Action != Save || out.Gen != 1 || out.Editing != "" || c.Name != "weekly" || c.Kind != "agent" || c.Mode != "work" || c.Tool != "claude" ||
		c.RepoPath != "/src/repo" || c.Retention != DefaultRetention || c.Timezone != "Europe/Lisbon" || c.Entry != "prompts/weekly.md" {
		t.Fatalf("save = %+v", out)
	}
	if again := d.Key(tea.KeyMsg{Type: tea.KeyEnter}); again.Action != Refused {
		t.Fatalf("a second enter while saving = %+v", again)
	}
}

func TestChoicesStepByKeyAndByClick(t *testing.T) {
	d := New(Seed{Tools: []string{"claude"}}, 1)
	d.View(look())
	kindLine := -1
	for line, field := range d.lines {
		if field == KindField {
			kindLine = line
			break
		}
	}
	d.Click(kindLine)
	if d.Focus() != KindField || kinds[d.kind] != automations.KindAgent {
		t.Fatalf("the first click should focus kind, focus %d kind %s", d.Focus(), kinds[d.kind])
	}
	d.Click(kindLine)
	if kinds[d.kind] != automations.KindScript {
		t.Fatalf("a click on the focused kind left it %s", kinds[d.kind])
	}
	d.Key(tea.KeyMsg{Type: tea.KeyLeft})
	if kinds[d.kind] != automations.KindAgent {
		t.Fatalf("← left kind %s", kinds[d.kind])
	}
	d.Key(tea.KeyMsg{Type: tea.KeyTab})
	d.Key(tea.KeyMsg{Type: tea.KeyRight})
	if d.tools[d.tool] != "" {
		t.Fatalf("→ past the last CLI = %q; want none", d.tools[d.tool])
	}
}

func TestRefusalsLandOnTheirField(t *testing.T) {
	d := Edit("weekly", automations.Config{Name: "weekly", Retention: 3, Tool: "gone"}, []string{"claude"}, 4)
	if d.tools[d.tool] != "gone" {
		t.Fatalf("an unknown stored CLI was replaced by %q", d.tools[d.tool])
	}
	d.inputs[RetentionField].SetValue("many")
	if out := d.Key(tea.KeyMsg{Type: tea.KeyEnter}); out.Action != Refused || d.Refusal().Field != RetentionField || d.Focus() != RetentionField {
		t.Fatalf("a retention that is not a number: %+v, refusal %+v", out, d.Refusal())
	}
	d.inputs[RetentionField].SetValue("3")
	if out := d.Key(tea.KeyMsg{Type: tea.KeyEnter}); out.Action != Save || out.Editing != "weekly" {
		t.Fatalf("save = %+v", out)
	}
	d.Refuse(3, &automations.FieldError{Field: "cron", Reason: "is wrong"})
	if d.Refusal().Field == CronField {
		t.Fatal("a refusal for another dialog landed")
	}
	d.Refuse(4, &automations.FieldError{Field: "cron", Reason: "is wrong"})
	if r := d.Refusal(); r.Field != CronField || r.Reason != "cron is wrong" || d.Focus() != CronField {
		t.Fatalf("refusal = %+v, focus %d", r, d.Focus())
	}
	if body := d.View(look()).Body; !strings.Contains(body, "cron is wrong") {
		t.Fatalf("body:\n%s", body)
	}
	d.Refuse(4, errors.New("disk full"))
	if r := d.Refusal(); r.Field != -1 || r.Reason != "disk full" {
		t.Fatalf("a plain error = %+v", r)
	}
}
