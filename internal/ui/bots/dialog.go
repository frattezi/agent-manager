package bots

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	uidialogs "github.com/YoanWai/agent-manager/internal/ui/dialogs"
	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// The dialog's fields, in focus order: the order of Fields, each painted on
// its own body line.
const (
	NameField = iota
	EmojiField
	ColorField
	ToolField
	ModelField
	DirField
	FlagsField
	SpecializationField
	MemoryField
	fieldCount
)

var labels = [fieldCount]string{
	NameField: "name", EmojiField: "emoji", ColorField: "colour", ToolField: "CLI", ModelField: "model",
	DirField: "directory", FlagsField: "flags", SpecializationField: "skill", MemoryField: "memory",
}

// Catalog is the manager's capability discovery as the dialog reads it, the
// answer the spawn picker lists models from. The dialog asks it on every
// step and paint, so a CLI that answers later, or lists a new model, shows
// without reopening.
type Catalog interface {
	Models(tool string) Models
	// Load asks the CLI for its models off the update path; nil when its
	// answer is kept or it has no catalog.
	Load(tool string) tea.Cmd
}

// Models are the model keys a CLI lists, and Note says why there are none
// to pick from: still reading, unreadable, or a CLI that takes no model.
type Models struct {
	Keys []string
	Note string
}

// Seed is what a new Profile starts from: the directory the root suggests,
// the CLIs a Bot may run and how many swatches the theme hands out.
type Seed struct {
	Dir      string
	Tools    []string
	Swatches int
}

// Dialog creates a Bot Profile, or edits the one Editing names.
type Dialog struct {
	inputs   [fieldCount]textinput.Model
	color    int
	swatches int
	tools    []string
	tool     int
	// model is the picked key, empty for the CLI's own default; it is kept
	// by value since the list it is picked from follows discovery.
	model   string
	catalog Catalog
	focus   int
	editing string
	refusal Refusal
	saving  bool
	gen     uint64
	// lines is the field each body line the last View painted belongs to.
	lines []int
}

// Refusal is a reason a save was refused, shown under Field, or under the
// fields when Field is -1.
type Refusal struct {
	Field  int
	Reason string
}

// New opens a dialog on a new Profile.
func New(seed Seed, catalog Catalog, gen uint64) Dialog {
	p := Profile{Dir: seed.Dir}
	if len(seed.Tools) > 0 {
		p.Tool = seed.Tools[0]
	}
	return open(p, "", seed, catalog, gen)
}

// Edit opens a dialog on the stored Profile p.
func Edit(p Profile, seed Seed, catalog Catalog, gen uint64) Dialog {
	return open(p, p.ID, seed, catalog, gen)
}

func open(p Profile, editing string, seed Seed, catalog Catalog, gen uint64) Dialog {
	d := Dialog{editing: editing, gen: gen, catalog: catalog, swatches: max(seed.Swatches, 1), refusal: Refusal{Field: -1}}
	d.tools = slices.Clone(seed.Tools)
	if p.Tool != "" && !slices.Contains(d.tools, p.Tool) {
		// A CLI the manager no longer knows stays shown, so the refusal
		// explains it instead of the dialog silently picking another.
		d.tools = append(d.tools, p.Tool)
	}
	d.tool = max(slices.Index(d.tools, p.Tool), 0)
	d.color = p.Color
	d.model = p.Model
	placeholders := map[int]string{
		NameField: "operator", EmojiField: "an emoji its rows carry", DirField: "/path/a/bot/starts/in",
		FlagsField: "launch flags, space separated", SpecializationField: "the skill a Bot loads first",
		MemoryField: "the ai-memory project its Bots share",
	}
	for field := range fieldCount {
		if choice(field) {
			continue
		}
		in := textinput.New()
		in.Placeholder = placeholders[field]
		in.CharLimit = 1024
		in.SetValue(p.Get(Fields[field].Key))
		in.CursorEnd()
		d.inputs[field] = in
	}
	d.focusOn(NameField)
	return d
}

func choice(field int) bool { return field == ColorField || field == ToolField || field == ModelField }

func (d *Dialog) Gen() uint64 { return d.gen }

func (d *Dialog) Focus() int { return d.focus }

// Editing is the id of the Profile being edited; empty creates one.
func (d *Dialog) Editing() string { return d.editing }

func (d *Dialog) Refusal() Refusal { return d.refusal }

// Tool is the CLI the dialog has picked, whose models it lists.
func (d *Dialog) Tool() string {
	if len(d.tools) == 0 {
		return ""
	}
	return d.tools[d.tool]
}

// Load asks the picked CLI for its models; the root runs it on open.
func (d *Dialog) Load() tea.Cmd { return d.catalog.Load(d.Tool()) }

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
	var refused *FieldError
	if !errors.As(err, &refused) {
		d.refusal = Refusal{Field: -1, Reason: err.Error()}
		return
	}
	at := slices.IndexFunc(Fields, func(f Field) bool { return f.Key == refused.Key })
	if at < 0 {
		d.refusal = Refusal{Field: -1, Reason: err.Error()}
		return
	}
	d.refusal = Refusal{Field: at, Reason: labels[at] + " " + refused.Reason}
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
	Profile Profile
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
			return Outcome{Cmd: d.step(1)}
		case "left":
			return Outcome{Cmd: d.step(-1)}
		}
		return Outcome{}
	}
	var cmd tea.Cmd
	d.inputs[d.focus], cmd = d.inputs[d.focus].Update(msg)
	return Outcome{Cmd: cmd}
}

// Click focuses the field painted on the body's line; a click on the
// focused choice steps it, as → does.
func (d *Dialog) Click(line int) tea.Cmd {
	if line < 0 || line >= len(d.lines) || d.lines[line] < 0 {
		return nil
	}
	field := d.lines[line]
	if field == d.focus && choice(field) {
		return d.step(1)
	}
	d.focusOn(field)
	return nil
}

// step moves the focused choice; a new CLI starts on its own default model
// and is asked for its models.
func (d *Dialog) step(delta int) tea.Cmd {
	switch d.focus {
	case ColorField:
		d.color = ((d.color+delta)%d.swatches + d.swatches) % d.swatches
	case ToolField:
		if len(d.tools) == 0 {
			return nil
		}
		d.tool = (d.tool + delta + len(d.tools)) % len(d.tools)
		d.model = ""
		return d.Load()
	case ModelField:
		options := d.modelOptions()
		at := max(slices.Index(options, d.model), 0)
		d.model = options[(at+delta+len(options))%len(options)]
	}
	return nil
}

// modelOptions are the CLI's default, then what discovery lists, then a
// stored model it no longer lists, kept so an edit does not drop it.
func (d *Dialog) modelOptions() []string {
	options := append([]string{""}, d.catalog.Models(d.Tool()).Keys...)
	if !slices.Contains(options, d.model) {
		options = append(options, d.model)
	}
	return options
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

func (d *Dialog) submit() Outcome {
	p := Profile{ID: d.editing, Color: d.color, Tool: d.Tool(), Model: d.model}
	for field := range fieldCount {
		if !choice(field) {
			// Set trims what each field trims; no text field refuses here.
			_ = p.Set(Fields[field].Key, d.inputs[field].Value())
		}
	}
	p.Dir = strings.TrimSpace(p.Dir)
	d.refusal = Refusal{Field: -1}
	d.saving = true
	return Outcome{Action: Save, Profile: p, Editing: d.editing, Gen: d.gen}
}

// Look is what the dialog paints with.
type Look struct {
	GlyphMode presentation.GlyphMode
	Field     func(label, value string, focused bool) string
	Input     func(textinput.Model) string
	// Swatch is the style of the theme's colour at an index.
	Swatch func(index int) lipgloss.Style
	Value  lipgloss.Style
	Subtle lipgloss.Style
	Muted  lipgloss.Style
	Err    lipgloss.Style
	// Width is the card's inner width, which a refusal wraps to.
	Width int
}

func (d *Dialog) View(look Look) uidialogs.Card {
	chrome := presentation.ChromeFor(look.GlyphMode)
	glyphs := presentation.GlyphsFor(look.GlyphMode)
	var body strings.Builder
	d.lines = d.lines[:0]
	add := func(text string, field int) {
		body.WriteString(text)
		for range strings.Count(text, "\n") {
			d.lines = append(d.lines, field)
		}
	}
	models := d.catalog.Models(d.Tool())
	for field := range fieldCount {
		var value string
		switch field {
		case ColorField:
			value = d.choiceView(look, chrome, look.Swatch(d.color).Render(glyphs.FinishedGlyph+" "+strconv.Itoa(d.color)))
		case ToolField:
			value = d.choiceView(look, chrome, look.Value.Render(d.Tool()))
		case ModelField:
			name := d.model
			if name == "" {
				name = "cli default"
			}
			value = d.choiceView(look, chrome, look.Value.Render(name))
			if len(models.Keys) == 0 && models.Note != "" {
				value += "  " + look.Subtle.Render(models.Note)
			}
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
	add("\n"+look.Muted.Render("  The models are the ones the CLI lists. The skill is")+"\n"+
		look.Muted.Render("  the Specialization a Bot loads first.")+"\n", -1)
	hints := [][2]string{{presentation.NavigationLabel(look.GlyphMode, "tab/↑↓"), "move"}}
	if choice(d.focus) {
		hints = append(hints, [2]string{presentation.NavigationLabel(look.GlyphMode, "←→"), "change"})
	}
	hints = append(hints, [2]string{"↵", "save"}, [2]string{"esc", "cancel"})
	title := glyphs.NewGlyph + " New Bot Profile"
	if d.editing != "" {
		title = glyphs.GroupOpenGlyph + " Edit Bot Profile"
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
	return look.Subtle.Render(chrome.ChoiceLeftGlyph+" ") + value + look.Subtle.Render(" "+chrome.ChoiceRightGlyph)
}
