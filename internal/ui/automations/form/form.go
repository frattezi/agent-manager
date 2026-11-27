// Package form is the Automations dialog: the fields an Automation is made
// of, their keys, clicks and view. It builds a configuration and hands it to
// the root, which stores it through the Automations commands; the refusal
// those commands return is shown against the field it names.
package form

import (
	"cmp"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/YoanWai/agent-manager/internal/ui/automations"
	uidialogs "github.com/YoanWai/agent-manager/internal/ui/dialogs"
	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Fields, in focus order; each paints on its own body line.
const (
	NameField = iota
	DirField
	KindField
	ToolField
	ModeField
	CronField
	TimezoneField
	EntryField
	FlagsField
	RetentionField
	fieldCount
)

// DefaultRetention is how many runs a new Automation keeps.
const DefaultRetention = 10

var (
	kinds = []string{automations.KindAgent, automations.KindScript}
	modes = []string{automations.ModeWork, automations.ModePersonal}
)

// fieldOf is the dialog field a configuration field's refusal is shown on.
var fieldOf = map[string]int{
	"name": NameField, "repoPath": DirField, "kind": KindField, "cli": ToolField, "mode": ModeField,
	"cron": CronField, "timezone": TimezoneField, "entryPoint": EntryField, "flags": FlagsField, "keepRuns": RetentionField,
}

// Seed is what a new dialog starts from: the directory and timezone the
// root suggests and the CLIs an Agent Automation may name.
type Seed struct {
	Dir      string
	Timezone string
	Tools    []string
}

// Dialog creates an Automation, or edits the one Editing names.
type Dialog struct {
	inputs [fieldCount]textinput.Model
	kind   int
	mode   int
	tool   int
	// tools are the CLI choices; the first is none, for a Script
	// Automation that names no CLI.
	tools   []string
	focus   int
	editing string
	refusal Refusal
	saving  bool
	gen     uint64
	// lines is the field each body line the last View painted belongs to.
	lines []int
}

// Refusal is a reason a save was refused, shown under Field.
type Refusal struct {
	Field  int
	Reason string
}

// New opens a dialog on a new Automation.
func New(seed Seed, gen uint64) Dialog {
	config := automations.Config{RepoPath: seed.Dir, Timezone: seed.Timezone, Kind: automations.KindAgent, Mode: automations.ModeWork, Retention: DefaultRetention}
	if len(seed.Tools) > 0 {
		config.Tool = seed.Tools[0]
	}
	return open(config, "", seed.Tools, gen)
}

// Edit opens a dialog on the stored configuration of the Automation id.
func Edit(id string, config automations.Config, tools []string, gen uint64) Dialog {
	return open(config, id, tools, gen)
}

func open(config automations.Config, editing string, tools []string, gen uint64) Dialog {
	d := Dialog{editing: editing, gen: gen, tools: append([]string{""}, tools...), refusal: Refusal{Field: -1}}
	if config.Tool != "" && !slices.Contains(d.tools, config.Tool) {
		// A CLI the manager no longer knows stays shown, so the refusal
		// explains it instead of the dialog silently picking another.
		d.tools = append(d.tools, config.Tool)
	}
	d.kind = max(slices.Index(kinds, config.Kind), 0)
	d.mode = max(slices.Index(modes, config.Mode), 0)
	d.tool = max(slices.Index(d.tools, config.Tool), 0)
	retention := ""
	if config.Retention > 0 {
		retention = strconv.Itoa(config.Retention)
	}
	values := map[int]string{
		NameField: config.Name, DirField: config.RepoPath, CronField: config.Cron, TimezoneField: config.Timezone,
		EntryField: config.Entry, FlagsField: strings.Join(config.Flags, " "), RetentionField: retention,
	}
	placeholders := map[int]string{
		NameField: "weekly report", DirField: "/path/to/a/fractory/repo", CronField: "0 7 * * 1",
		TimezoneField: "Europe/Lisbon", EntryField: "", FlagsField: "launch flags, space separated", RetentionField: "runs to keep",
	}
	for field := range fieldCount {
		if choice(field) {
			continue
		}
		in := textinput.New()
		in.Placeholder = placeholders[field]
		in.CharLimit = 1024
		in.SetValue(values[field])
		in.CursorEnd()
		d.inputs[field] = in
	}
	d.focusOn(NameField)
	return d
}

func choice(field int) bool { return field == KindField || field == ToolField || field == ModeField }

func (d *Dialog) Gen() uint64 { return d.gen }

func (d *Dialog) Focus() int { return d.focus }

// Editing is the id of the Automation being edited; empty creates one.
func (d *Dialog) Editing() string { return d.editing }

func (d *Dialog) Refusal() Refusal { return d.refusal }

// SyncWidths fits the text fields to the card, so a long value scrolls.
func (d *Dialog) SyncWidths(width int) {
	for field := range fieldCount {
		if !choice(field) {
			presentation.FitInput(&d.inputs[field], width)
		}
	}
}

// Refuse shows why the save of dialog gen was refused, against the field
// the reason names, and takes enter again.
func (d *Dialog) Refuse(gen uint64, err error) {
	if d.gen != gen {
		return
	}
	d.saving = false
	var field *automations.FieldError
	if !errors.As(err, &field) {
		d.refusal = Refusal{Field: -1, Reason: err.Error()}
		return
	}
	at, ok := fieldOf[field.Field]
	if !ok {
		at = NameField
	}
	d.refusal = Refusal{Field: at, Reason: labels[at] + " " + field.Reason}
	d.focusOn(at)
}

type Action uint8

const (
	None Action = iota
	Cancel
	Save
	Refused
)

// Outcome is what a key or a click asks of the root.
type Outcome struct {
	Action  Action
	Cmd     tea.Cmd
	Config  automations.Config
	Editing string
	Gen     uint64
}

func (d *Dialog) Key(msg tea.KeyMsg) Outcome {
	switch msg.String() {
	case "esc":
		return Outcome{Action: Cancel}
	case "tab", "down":
		d.focusOn((d.focus + 1) % fieldCount)
		return Outcome{}
	case "shift+tab", "up":
		d.focusOn((d.focus + fieldCount - 1) % fieldCount)
		return Outcome{}
	case "enter":
		if d.saving {
			return Outcome{Action: Refused}
		}
		return d.submit()
	}
	if choice(d.focus) {
		switch msg.String() {
		case "right", " ", "space":
			d.step(1)
		case "left":
			d.step(-1)
		}
		return Outcome{}
	}
	var cmd tea.Cmd
	d.inputs[d.focus], cmd = d.inputs[d.focus].Update(msg)
	return Outcome{Cmd: cmd}
}

// Click focuses the field painted on the body's line; a click on the
// focused choice steps it, as → does.
func (d *Dialog) Click(line int) {
	if line < 0 || line >= len(d.lines) || d.lines[line] < 0 {
		return
	}
	field := d.lines[line]
	if field == d.focus && choice(field) {
		d.step(1)
		return
	}
	d.focusOn(field)
}

func (d *Dialog) step(delta int) {
	switch d.focus {
	case KindField:
		d.kind = (d.kind + delta + len(kinds)) % len(kinds)
	case ModeField:
		d.mode = (d.mode + delta + len(modes)) % len(modes)
	case ToolField:
		d.tool = (d.tool + delta + len(d.tools)) % len(d.tools)
	}
}

func (d *Dialog) focusOn(field int) {
	d.focus = field
	for index := range fieldCount {
		if !choice(index) {
			d.inputs[index].Blur()
		}
	}
	if !choice(field) {
		d.inputs[field].Focus()
	}
}

func (d *Dialog) value(field int) string { return strings.TrimSpace(d.inputs[field].Value()) }

func (d *Dialog) submit() Outcome {
	config := automations.Config{
		Version:  automations.Version,
		Name:     d.value(NameField),
		Kind:     kinds[d.kind],
		RepoPath: d.value(DirField),
		Cron:     d.value(CronField),
		Timezone: d.value(TimezoneField),
		Mode:     modes[d.mode],
		Tool:     d.tools[d.tool],
		Flags:    strings.Fields(d.value(FlagsField)),
		Entry:    d.value(EntryField),
	}
	retention, err := strconv.Atoi(d.value(RetentionField))
	if err != nil {
		d.refusal = Refusal{Field: RetentionField, Reason: "retention " + strconv.Quote(d.value(RetentionField)) + " is not a whole number"}
		d.focusOn(RetentionField)
		return Outcome{Action: Refused}
	}
	config.Retention = retention
	d.refusal = Refusal{Field: -1}
	d.saving = true
	return Outcome{Action: Save, Config: config, Editing: d.editing, Gen: d.gen}
}

// Look is what the dialog paints with.
type Look struct {
	GlyphMode presentation.GlyphMode
	Field     func(label, value string, focused bool) string
	Input     func(textinput.Model) string
	Value     lipgloss.Style
	Subtle    lipgloss.Style
	Muted     lipgloss.Style
	Err       lipgloss.Style
	// Width is the card's inner width, which a refusal wraps to.
	Width int
}

var labels = [fieldCount]string{
	NameField: "name", DirField: "directory", KindField: "kind", ToolField: "CLI", ModeField: "mode",
	CronField: "cron", TimezoneField: "timezone", EntryField: "entry", FlagsField: "flags", RetentionField: "keep runs",
}

func (d *Dialog) View(look Look) uidialogs.Card {
	chrome := presentation.ChromeFor(look.GlyphMode)
	var body strings.Builder
	d.lines = d.lines[:0]
	add := func(text string, field int) {
		body.WriteString(text)
		for range strings.Count(text, "\n") {
			d.lines = append(d.lines, field)
		}
	}
	entryHint := "prompts/weekly.md, the prompt the agent is given"
	if kinds[d.kind] == automations.KindScript {
		entryHint = "scripts/weekly.sh, the committed script it runs"
	}
	d.inputs[EntryField].Placeholder = entryHint
	for field := range fieldCount {
		var value string
		switch field {
		case KindField:
			value = d.choiceView(look, chrome, kinds[d.kind])
		case ModeField:
			value = d.choiceView(look, chrome, modes[d.mode])
		case ToolField:
			value = d.choiceView(look, chrome, cmp.Or(d.tools[d.tool], "none"))
		default:
			value = look.Input(d.inputs[field])
		}
		add(look.Field(labels[field], value, d.focus == field), field)
		if d.refusal.Field == field {
			add(d.wrapped(look, d.refusal.Reason), field)
		}
	}
	if d.refusal.Field < 0 && d.refusal.Reason != "" {
		add("\n"+d.wrapped(look, d.refusal.Reason), -1)
	}
	add("\n"+look.Muted.Render("  The directory needs a .fractory.json, and the cron")+"\n"+
		look.Muted.Render("  is read in the timezone.")+"\n", -1)
	hints := [][2]string{{presentation.NavigationLabel(look.GlyphMode, "tab/↑↓"), "move"}}
	if choice(d.focus) {
		hints = append(hints, [2]string{presentation.NavigationLabel(look.GlyphMode, "←→"), "change"})
	}
	hints = append(hints, [2]string{"↵", "save"}, [2]string{"esc", "cancel"})
	glyphs := presentation.GlyphsFor(look.GlyphMode)
	title := glyphs.NewGlyph + " New Automation"
	if d.editing != "" {
		title = glyphs.GroupOpenGlyph + " Edit Automation"
	}
	return uidialogs.Card{Title: title, Body: strings.TrimRight(body.String(), "\n"), Hints: hints}
}

// wrapped is a refusal set in the error tone, indented and wrapped to the
// card so a long path does not cut the reason off.
func (d *Dialog) wrapped(look Look, reason string) string {
	var b strings.Builder
	for _, line := range strings.Split(ansi.Wordwrap(reason, max(look.Width-4, 20), "/-"), "\n") {
		b.WriteString(look.Err.Render("    "+line) + "\n")
	}
	return b.String()
}

func (d *Dialog) choiceView(look Look, chrome presentation.ChromeSet, value string) string {
	return look.Subtle.Render(chrome.ChoiceLeftGlyph+" ") + look.Value.Render(value) + look.Subtle.Render(" "+chrome.ChoiceRightGlyph)
}
