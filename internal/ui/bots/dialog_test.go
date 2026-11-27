package bots

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// fakeCatalog answers as the manager's discovery would, and records which
// CLIs the dialog asked.
type fakeCatalog struct {
	models map[string][]string
	asked  []string
}

func (c *fakeCatalog) Models(tool string) Models {
	if keys := c.models[tool]; len(keys) > 0 {
		return Models{Keys: keys}
	}
	return Models{Note: "reading models from " + tool + "…"}
}

func (c *fakeCatalog) Load(tool string) tea.Cmd {
	c.asked = append(c.asked, tool)
	return func() tea.Msg { return nil }
}

func look() Look {
	return Look{
		Field:  func(label, value string, focused bool) string { return label + " " + value + "\n" },
		Input:  func(in textinput.Model) string { return in.Value() },
		Swatch: func(int) lipgloss.Style { return lipgloss.NewStyle() },
		Width:  60, Value: lipgloss.NewStyle(), Subtle: lipgloss.NewStyle(), Muted: lipgloss.NewStyle(), Err: lipgloss.NewStyle(),
	}
}

var seed = Seed{Dir: "/src/fractory", Tools: []string{"claude", "codex"}, Swatches: 3}

func stored() Profile {
	return Profile{
		ID: "p1", Name: "operator", Emoji: "🛠", Color: 2, Tool: "codex", Model: "gpt-5", Dir: "/src/ops",
		Flags: "--yolo", Specialization: "fractory-operator", MemoryProject: "fractory-ops",
	}
}

func enter() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEnter} }

func TestANewDialogOpensEmptyOnTheSeed(t *testing.T) {
	d := New(seed, &fakeCatalog{}, 1)
	for field := range fieldCount {
		if !choice(field) && field != DirField && d.inputs[field].Value() != "" {
			t.Errorf("%s opens on %q, want empty", labels[field], d.inputs[field].Value())
		}
	}
	if d.Editing() != "" || d.Tool() != "claude" || d.model != "" || d.color != 0 || d.inputs[DirField].Value() != "/src/fractory" {
		t.Fatalf("new dialog: editing %q tool %q model %q color %d dir %q", d.Editing(), d.Tool(), d.model, d.color, d.inputs[DirField].Value())
	}
	if title := d.View(look()).Title; !strings.Contains(title, "New Bot Profile") {
		t.Fatalf("title %q", title)
	}
	d.inputs[NameField].SetValue("  watcher ")
	out := d.Key(enter())
	if out.Action != Save || out.Editing != "" || out.Gen != 1 || out.Profile.Name != "watcher" || out.Profile.Tool != "claude" || out.Profile.ID != "" {
		t.Fatalf("save = %+v", out)
	}
	if again := d.Key(enter()); again.Action != Refused {
		t.Fatalf("a second enter while saving = %+v", again)
	}
}

func TestAnEditOpensOnTheStoredProfileAndKeepsItsID(t *testing.T) {
	want := stored()
	d := Edit(want, seed, &fakeCatalog{models: map[string][]string{"codex": {"gpt-5", "gpt-5-mini"}}}, 2)
	if title := d.View(look()).Title; !strings.Contains(title, "Edit Bot Profile") {
		t.Fatalf("title %q", title)
	}
	out := d.Key(enter())
	if out.Action != Save || out.Editing != "p1" || out.Profile != want {
		t.Fatalf("an untouched edit saves %+v, want %+v", out.Profile, want)
	}
}

func TestFieldsMoveByTabAndArrowsAndChoicesStep(t *testing.T) {
	d := New(seed, &fakeCatalog{}, 1)
	d.Key(tea.KeyMsg{Type: tea.KeyTab})
	d.Key(tea.KeyMsg{Type: tea.KeyDown})
	if d.Focus() != ColorField {
		t.Fatalf("tab, ↓ landed on %d", d.Focus())
	}
	d.Key(tea.KeyMsg{Type: tea.KeyLeft})
	if d.color != 2 {
		t.Fatalf("← from the first swatch = %d, want the last of 3", d.color)
	}
	d.Key(tea.KeyMsg{Type: tea.KeyShiftTab})
	d.Key(tea.KeyMsg{Type: tea.KeyUp})
	if d.Focus() != NameField {
		t.Fatalf("shift+tab, ↑ landed on %d", d.Focus())
	}
	d.Key(tea.KeyMsg{Type: tea.KeyUp})
	if d.Focus() != MemoryField {
		t.Fatalf("↑ from the first field = %d, want it to wrap to the last", d.Focus())
	}
	if out := d.Key(tea.KeyMsg{Type: tea.KeyEsc}); out.Action != Cancel {
		t.Fatalf("esc = %+v", out)
	}
}

func TestAClickFocusesAFieldAndStepsAFocusedChoice(t *testing.T) {
	catalog := &fakeCatalog{}
	d := New(seed, catalog, 1)
	d.View(look())
	toolLine := slices.Index(d.lines, ToolField)
	if cmd := d.Click(toolLine); cmd != nil || d.Focus() != ToolField || d.Tool() != "claude" {
		t.Fatalf("the first click should focus the CLI: focus %d tool %s", d.Focus(), d.Tool())
	}
	if cmd := d.Click(toolLine); cmd == nil || d.Tool() != "codex" {
		t.Fatalf("a click on the focused CLI left %s, cmd %v", d.Tool(), cmd)
	}
	if !slices.Equal(catalog.asked, []string{"codex"}) {
		t.Fatalf("asked %v for models, want the CLI stepped to", catalog.asked)
	}
}

func TestTheModelListFollowsDiscovery(t *testing.T) {
	catalog := &fakeCatalog{models: map[string][]string{}}
	d := New(seed, catalog, 1)
	for d.Focus() != ModelField {
		d.Key(tea.KeyMsg{Type: tea.KeyTab})
	}
	if body := d.View(look()).Body; !strings.Contains(body, "reading models from claude") {
		t.Fatalf("before the CLI answers:\n%s", body)
	}
	d.Key(tea.KeyMsg{Type: tea.KeyRight})
	if d.model != "" {
		t.Fatalf("→ with nothing discovered picked %q", d.model)
	}
	catalog.models["claude"] = []string{"opus", "sonnet"}
	d.Key(tea.KeyMsg{Type: tea.KeyRight})
	d.Key(tea.KeyMsg{Type: tea.KeyRight})
	if d.model != "sonnet" {
		t.Fatalf("→ → once claude answered = %q, want sonnet", d.model)
	}
	catalog.models["claude"] = []string{"opus", "sonnet", "haiku-next"}
	d.Key(tea.KeyMsg{Type: tea.KeyRight})
	if d.model != "haiku-next" {
		t.Fatalf("a model claude lists later = %q, want haiku-next", d.model)
	}
	d.Key(tea.KeyMsg{Type: tea.KeyRight})
	if d.model != "" {
		t.Fatalf("→ past the last model = %q, want the CLI's default", d.model)
	}
}

func TestAStoredModelTheCLINoLongerListsIsKept(t *testing.T) {
	d := Edit(stored(), seed, &fakeCatalog{models: map[string][]string{"codex": {"gpt-6"}}}, 1)
	if got := d.modelOptions(); !slices.Equal(got, []string{"", "gpt-6", "gpt-5"}) {
		t.Fatalf("options %v", got)
	}
	for d.Focus() != ToolField {
		d.Key(tea.KeyMsg{Type: tea.KeyTab})
	}
	d.Key(tea.KeyMsg{Type: tea.KeyRight})
	if d.Tool() != "claude" || d.model != "" {
		t.Fatalf("a new CLI = %s on %q, want its own default", d.Tool(), d.model)
	}
}

func TestARefusalIsShownAgainstTheFieldItNames(t *testing.T) {
	d := Edit(Profile{ID: "p1", Name: "operator", Tool: "gone"}, seed, &fakeCatalog{}, 4)
	if d.Tool() != "gone" {
		t.Fatalf("an unknown stored CLI was replaced by %q", d.Tool())
	}
	d.Key(enter())
	d.Refuse(3, &FieldError{Key: ToolKey, Reason: "is wrong"})
	if d.Refusal().Field == ToolField {
		t.Fatal("a refusal for another dialog landed")
	}
	err := Profile{Name: "operator", Tool: "gone"}.AdmitAmong(seed.Tools, seed.Swatches)
	d.Refuse(4, err)
	if r := d.Refusal(); r.Field != ToolField || !strings.HasPrefix(r.Reason, "CLI takes one of claude, codex") || d.Focus() != ToolField {
		t.Fatalf("refusal = %+v, focus %d", r, d.Focus())
	}
	body := d.View(look()).Body
	lines := strings.Split(body, "\n")
	at := slices.IndexFunc(lines, func(line string) bool { return strings.HasPrefix(line, "CLI ") })
	if at < 0 || !strings.Contains(lines[at+1], "CLI takes one of") {
		t.Fatalf("the reason is not under the CLI field:\n%s", body)
	}
	if out := d.Key(enter()); out.Action != Save {
		t.Fatalf("a refused dialog should take enter again: %+v", out)
	}
	d.Refuse(4, errors.New("disk full"))
	if r := d.Refusal(); r.Field != -1 || r.Reason != "disk full" {
		t.Fatalf("a plain error = %+v", r)
	}
}
