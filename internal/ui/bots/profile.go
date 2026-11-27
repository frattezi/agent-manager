// Package bots is the Bot Profile: the fields an owner sets and the values
// each takes, so agent-manager bots and the manager write the same ones.
package bots

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The keys a Profile's fields carry in ui.SettingSpecs.
const (
	NameKey           = "bot.name"
	EmojiKey          = "bot.emoji"
	ColorKey          = "bot.color"
	ToolKey           = "bot.tool"
	ModelKey          = "bot.model"
	DirKey            = "bot.dir"
	FlagsKey          = "bot.flags"
	SpecializationKey = "bot.specialization"
	MemoryKey         = "bot.memory"
)

// Field is one field of a Profile as the shell names it.
type Field struct {
	Key   string
	Flag  string
	About string
}

// Fields are a Profile's fields in the order they are shown.
var Fields = []Field{
	{NameKey, "name", "what the owner calls the Profile"},
	{EmojiKey, "emoji", "the emoji its rows carry"},
	{ColorKey, "color", "the index of its colour among the theme's swatches"},
	{ToolKey, "tool", "the CLI a Bot runs"},
	{ModelKey, "model", "the model a Bot runs on; empty leaves the CLI's own default"},
	{DirKey, "dir", "the directory a Bot starts in"},
	{FlagsKey, "flags", "the flags a Bot's CLI launches with"},
	{SpecializationKey, "specialization", "the skill a Bot loads first"},
	{MemoryKey, "memory", "the ai-memory project every Bot from the Profile remembers in"},
}

// Spec is one Profile field and the values it takes; the root hands the
// list to the shell as store.SettingSpec.
type Spec struct {
	Key     string
	Values  []string
	Default string
}

// Specs is every Profile field. tools are the agent CLIs and swatches the
// number of colours every theme hands out. The model is discovered from the
// CLI when a Bot is picked, so it takes any text here.
func Specs(tools []string, swatches int) []Spec {
	colors := make([]string, swatches)
	for i := range colors {
		colors[i] = strconv.Itoa(i)
	}
	specs := make([]Spec, 0, len(Fields))
	for _, field := range Fields {
		spec := Spec{Key: field.Key}
		switch field.Key {
		case ColorKey:
			spec.Values, spec.Default = colors, "0"
		case ToolKey:
			spec.Values = tools
		}
		specs = append(specs, spec)
	}
	return specs
}

// Profile is a Bot Profile as the commands read and write it.
type Profile struct {
	ID             string
	Name           string
	Emoji          string
	Color          int
	Tool           string
	Model          string
	Dir            string
	Flags          string
	Specialization string
	MemoryProject  string
}

// Get is the field key as text.
func (p Profile) Get(key string) string {
	switch key {
	case NameKey:
		return p.Name
	case EmojiKey:
		return p.Emoji
	case ColorKey:
		return strconv.Itoa(p.Color)
	case ToolKey:
		return p.Tool
	case ModelKey:
		return p.Model
	case DirKey:
		return p.Dir
	case FlagsKey:
		return p.Flags
	case SpecializationKey:
		return p.Specialization
	case MemoryKey:
		return p.MemoryProject
	}
	return ""
}

// Set writes the field key from text; whether the value is one the field
// takes is its Spec's to say.
func (p *Profile) Set(key, value string) error {
	switch key {
	case NameKey:
		p.Name = strings.TrimSpace(value)
	case EmojiKey:
		p.Emoji = strings.TrimSpace(value)
	case ColorKey:
		color, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s takes a swatch index, not %q", key, value)
		}
		p.Color = color
	case ToolKey:
		p.Tool = value
	case ModelKey:
		p.Model = strings.TrimSpace(value)
	case DirKey:
		p.Dir = value
	case FlagsKey:
		p.Flags = strings.TrimSpace(value)
	case SpecializationKey:
		p.Specialization = strings.TrimSpace(value)
	case MemoryKey:
		p.MemoryProject = strings.TrimSpace(value)
	default:
		return fmt.Errorf("a Bot Profile has no field %s", key)
	}
	return nil
}

// Admit refuses a Profile no Bot could start from, or text a row cannot
// draw, naming the field.
func (p Profile) Admit() error {
	if p.Name == "" || utf8.RuneCountInString(p.Name) > 60 {
		return fmt.Errorf("%s needs 1 to 60 characters", NameKey)
	}
	if p.Tool == "" {
		return fmt.Errorf("%s is required: a Bot needs a CLI to run", ToolKey)
	}
	for _, field := range Fields {
		value := p.Get(field.Key)
		if !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) {
			return fmt.Errorf("%s cannot hold control characters", field.Key)
		}
	}
	return nil
}

// ErrAmbiguous reports a name more than one Profile carries.
var ErrAmbiguous = errors.New("more than one Bot Profile carries that name")

// Find takes an id, or a name exactly one Profile carries.
func Find(profiles []Profile, key string) (Profile, error) {
	var named []Profile
	for _, p := range profiles {
		if p.ID == key {
			return p, nil
		}
		if p.Name == key {
			named = append(named, p)
		}
	}
	switch len(named) {
	case 0:
		return Profile{}, fmt.Errorf("no Bot Profile has the id or name %q; agent-manager bots list names them", key)
	case 1:
		return named[0], nil
	}
	ids := make([]string, len(named))
	for i, p := range named {
		ids[i] = p.ID
	}
	return Profile{}, fmt.Errorf("%q: %w; use one of the ids %s", key, ErrAmbiguous, strings.Join(ids, ", "))
}
