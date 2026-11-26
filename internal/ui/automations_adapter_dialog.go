package ui

import (
	"time"

	"github.com/YoanWai/agent-manager/internal/keybind"
	"github.com/YoanWai/agent-manager/internal/ui/automations"
	uiautomationsform "github.com/YoanWai/agent-manager/internal/ui/automations/form"
	uidialogs "github.com/YoanWai/agent-manager/internal/ui/dialogs"
	uiframe "github.com/YoanWai/agent-manager/internal/ui/frame"
	uirail "github.com/YoanWai/agent-manager/internal/ui/rail"
	"github.com/YoanWai/agent-manager/internal/ui/spawn"
	uitheme "github.com/YoanWai/agent-manager/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// The Automations dialog and the row actions. Every write goes through the
// commands in internal/ui/automations on the effect lane, the ones
// agent-manager automations runs, and reads the section back when it lands.

// openAutomationDialog opens the dialog on a new Automation, or on the
// stored configuration of the one editing names.
func (m *Model) openAutomationDialog(editing *automations.Entry) {
	gen := uint64(m.nextComposerGen())
	tools := m.services.cfg.AgentToolNames()
	dialog := uiautomationsform.New(uiautomationsform.Seed{Dir: m.capturedGroupDefaultDir(""), Timezone: m.workspace.automations.zone, Tools: tools}, gen)
	if editing != nil {
		dialog = uiautomationsform.Edit(editing.ID, editing.Config, tools, gen)
	}
	m.push(uiautomationsform.NewSurface(automationsFormRoot{m}, dialog))
	m.clearErr()
}

// automationsFormRoot is the root as the dialog reaches it.
type automationsFormRoot struct{ *Model }

func (r automationsFormRoot) Look() uiautomationsform.Look {
	styles := r.spawnStyles()
	return uiautomationsform.Look{
		GlyphMode: r.prefs.glyphMode,
		Field: func(label, value string, focused bool) string {
			return spawn.FormField(styles, label, value, focused)
		},
		Input:  uiframe.TextInputView,
		Value:  uitheme.ValueStyle(),
		Subtle: uitheme.SubtleStyle(),
		Muted:  uitheme.MutedStyle(),
		Err:    uitheme.ErrStyle(),
	}
}

func (r automationsFormRoot) Save(gen uint64, editing string, config automations.Config) tea.Cmd {
	r.clearErr()
	r.enqueueEffect(automations.SaveRequest{Gen: gen, Editing: editing, Config: config, Tools: r.services.cfg.AgentToolNames(), Now: time.Now()}, 0, false)
	return r.nextEffectCmd()
}

// automationRowIntent runs an action on the Automation under the cursor:
// edit, enable or disable, run now and delete.
func (m *Model) automationRowIntent(kind uirail.ActionKind) bool {
	entry, ok := m.selectedAutomation()
	if !ok {
		return false
	}
	switch kind {
	case uirail.RenameAction:
		if entry.Err != nil {
			m.reportErr("this Automation cannot be read, so it cannot be edited here: " + entry.Err.Error())
			return true
		}
		m.openAutomationDialog(&entry)
	case uirail.ToggleEnabled:
		m.clearErr()
		m.enqueueEffect(automations.EnableRequest{ID: entry.ID, Enabled: !entry.Enabled, Now: time.Now()}, 0, false)
	case uirail.RunNow:
		m.clearErr()
		m.enqueueEffect(automations.RunNowRequest{ID: entry.ID, Now: time.Now()}, 0, false)
	case uirail.Delete:
		m.openConfirm(confirmTarget{
			prompt:     uidialogs.Confirm{Action: uidialogs.ConfirmDeleteAutomation, Label: "delete Automation " + entry.Name + "? It stops firing; the records of its past runs stay."},
			automation: entry.ID,
		})
	default:
		return false
	}
	return true
}

// automationManageHint names the keys an Automation row takes, for a key
// that does nothing on one.
func (m *Model) automationManageHint() string {
	return "an Automation takes " + m.listGlyph(keybind.Rename) + " edit, " + m.listGlyph(keybind.EnableToggle) + " enable / disable, " +
		m.listGlyph(keybind.RunNow) + " run now and " + m.listGlyph(keybind.Delete) + " delete"
}

// applyAutomationSave closes the dialog that saved and lists the stored
// Automation under the cursor, or shows the refusal against its field.
func (m *Model) applyAutomationSave(request automations.SaveRequest, result automations.SaveResult, err error) tea.Cmd {
	s, open := surfaceOf[*uiautomationsform.Surface](m)
	open = open && s.Dialog().Gen() == request.Gen
	if err != nil {
		if open {
			s.Dialog().Refuse(request.Gen, err)
		} else {
			m.reportErr("the Automation was not saved: " + err.Error())
		}
		return nil
	}
	if open {
		m.Pop()
	}
	m.showAutomations(result.Rows, result.Records)
	m.rail.Focus(uirail.Selection{Kind: uirail.AutomationRow, Automation: result.Config.ID})
	verb := "added "
	if request.Editing != "" {
		verb = "saved "
	}
	m.reportDone(verb + result.Config.Name)
	return nil
}

// applyAutomationWrite lists the section a toggle or a delete left.
func (m *Model) applyAutomationWrite(listing automations.Listing, done string, err error) tea.Cmd {
	if err != nil {
		m.reportErr(err.Error())
		return nil
	}
	m.showAutomations(listing.Rows, listing.Records)
	m.reportDone(done)
	return nil
}

func (m *Model) applyAutomationEnable(request automations.EnableRequest, listing automations.Listing, err error) tea.Cmd {
	done := "disabled " + m.automationName(listing, request.ID)
	if request.Enabled {
		done = "enabled " + m.automationName(listing, request.ID)
	}
	return m.applyAutomationWrite(listing, done, err)
}

func (m *Model) applyAutomationRemove(request automations.RemoveRequest, listing automations.Listing, err error) tea.Cmd {
	return m.applyAutomationWrite(listing, "deleted "+m.automationName(automations.Listing{}, request.ID), err)
}

// automationName is the name the listing, or else the section as shown,
// gives the Automation id.
func (m *Model) automationName(listing automations.Listing, id string) string {
	for _, row := range listing.Rows {
		if row.ID == id {
			return row.Name
		}
	}
	for _, entry := range m.workspace.automations.entries {
		if entry.ID == id {
			return entry.Name
		}
	}
	return id
}

func (m *Model) applyAutomationRunNow(request automations.RunNowRequest, result automations.RunNowResult, err error) tea.Cmd {
	name := m.automationName(automations.Listing{}, request.ID)
	switch {
	case err != nil:
		m.reportErr(name + " did not run: " + err.Error())
	case result.Skipped:
		m.reportWarn(name + " is still working on an earlier firing, so run now was skipped")
	default:
		m.reportDone("asked Fractory to run " + name + " for " + firingClock(result.FiredFor))
	}
	return nil
}
