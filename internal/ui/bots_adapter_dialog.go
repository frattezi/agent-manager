package ui

import (
	"time"

	uibots "github.com/YoanWai/agent-manager/internal/ui/bots"
	uiframe "github.com/YoanWai/agent-manager/internal/ui/frame"
	"github.com/YoanWai/agent-manager/internal/ui/spawn"
	uitheme "github.com/YoanWai/agent-manager/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The Bots dialog. A save goes through uibots.SaveRequest on the effect
// lane, which holds the Profile to the values agent-manager bots takes.

// openBotDialog opens the dialog on a new Profile, or on the stored one
// editing names, and asks its CLI for the models it lists.
func (m *Model) openBotDialog(editing *uibots.Profile) tea.Cmd {
	gen := uint64(m.nextComposerGen())
	seed := uibots.Seed{Dir: m.capturedGroupDefaultDir(""), Tools: m.services.cfg.AgentToolNames(), Swatches: len(uitheme.Current().Swatches())}
	dialog := uibots.New(seed, botsCatalog{m}, gen)
	if editing != nil {
		dialog = uibots.Edit(*editing, seed, botsCatalog{m}, gen)
	}
	m.push(uibots.NewSurface(botsFormRoot{m}, dialog))
	m.clearErr()
	return dialog.Load()
}

// botsCatalog is the spawn picker's capability discovery as the dialog
// reads it: the answer the CLI gave, never a list kept here.
type botsCatalog struct{ *Model }

func (c botsCatalog) Models(tool string) uibots.Models {
	caps := c.spawnHost().ChoiceCaps(tool)
	if !caps.Models {
		return uibots.Models{Note: tool + " takes no model"}
	}
	if !caps.Catalog {
		return uibots.Models{Note: tool + " lists no models"}
	}
	answer := c.ledger.spawner.Answer(tool)
	switch {
	case answer == nil || !answer.Loaded:
		return uibots.Models{Note: "reading models from " + tool + "…"}
	case answer.Err != nil && len(answer.Cat.Models) == 0:
		return uibots.Models{Note: "couldn't read models from " + tool + ": " + answer.Err.Error()}
	}
	keys := make([]string, 0, len(answer.Cat.Models))
	for _, model := range answer.Cat.Models {
		keys = append(keys, model.Key())
	}
	if len(keys) == 0 {
		return uibots.Models{Note: tool + " lists no models"}
	}
	return uibots.Models{Keys: keys}
}

func (c botsCatalog) Load(tool string) tea.Cmd { return c.ensureCatalog(tool) }

// botsFormRoot is the root as the dialog reaches it.
type botsFormRoot struct{ *Model }

func (r botsFormRoot) Look() uibots.Look {
	styles := r.spawnStyles()
	swatches := uitheme.Current().Swatches()
	return uibots.Look{
		GlyphMode: r.prefs.glyphMode,
		Field: func(label, value string, focused bool) string {
			return spawn.FormField(styles, label, value, focused)
		},
		Input: uiframe.TextInputView,
		Swatch: func(index int) lipgloss.Style {
			// A stored index past this theme's swatches still paints.
			return lipgloss.NewStyle().Foreground(lipgloss.Color(swatches[(index%len(swatches)+len(swatches))%len(swatches)]))
		},
		Value:  uitheme.ValueStyle(),
		Subtle: uitheme.SubtleStyle(),
		Muted:  uitheme.MutedStyle(),
		Err:    uitheme.ErrStyle(),
	}
}

func (r botsFormRoot) Save(gen uint64, editing string, p uibots.Profile) tea.Cmd {
	r.clearErr()
	if p.Dir != "" {
		p.Dir = r.capturedAbsolutePath(p.Dir, "")
	}
	r.enqueueEffect(uibots.SaveRequest{
		Gen: gen, Editing: editing, Profile: p, Tools: r.services.cfg.AgentToolNames(),
		Swatches: len(uitheme.Current().Swatches()), Now: time.Now(),
	}, 0, false)
	return r.nextEffectCmd()
}

// applyBotSave closes the dialog that saved, or shows the refusal against
// its field.
func (m *Model) applyBotSave(request uibots.SaveRequest, stored uibots.Profile, err error) tea.Cmd {
	s, open := surfaceOf[*uibots.Surface](m)
	open = open && s.Dialog().Gen() == request.Gen
	if err != nil {
		if open {
			s.Dialog().Refuse(request.Gen, err)
		} else {
			m.reportErr("the Bot Profile was not saved: " + err.Error())
		}
		return nil
	}
	if open {
		m.Pop()
	}
	verb := "added "
	if request.Editing != "" {
		verb = "saved "
	}
	m.reportDone(verb + stored.Name)
	return nil
}
