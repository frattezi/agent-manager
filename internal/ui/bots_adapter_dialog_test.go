package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/YoanWai/agent-manager/internal/catalog"
	"github.com/YoanWai/agent-manager/internal/store"
	uibots "github.com/YoanWai/agent-manager/internal/ui/bots"
	uilaunch "github.com/YoanWai/agent-manager/internal/ui/spawn/launch"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func botDialog(t *testing.T, m *Model) *uibots.Surface {
	t.Helper()
	s, ok := surfaceOf[*uibots.Surface](m)
	if !ok {
		t.Fatal("the Bots dialog is not open")
	}
	return s
}

// typeBotField tabs to field and types value over what it holds.
func typeBotField(t *testing.T, m *Model, field int, value string) {
	t.Helper()
	for botDialog(t, m).Dialog().Focus() != field {
		m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyTab})
	}
	m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyCtrlU})
	m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyCtrlK})
	if value != "" {
		m.applyTestMsg(t, runeKey(value))
	}
}

func onlyBotProfile(t *testing.T, m *Model) store.BotProfile {
	t.Helper()
	all, err := m.services.store.BotProfiles()
	if err != nil || len(all) != 1 {
		t.Fatalf("stored %d Bot Profiles (%v); want one", len(all), err)
	}
	return all[0]
}

func TestBotDialogStoresAProfileAndEditsItInPlace(t *testing.T) {
	m := buildModel(t)
	m.openBotDialog(nil)
	if view := ansi.Strip(preparedView(m)); !strings.Contains(view, "New Bot Profile") {
		t.Fatalf("the dialog is not drawn:\n%s", view)
	}
	dir := t.TempDir()
	typeBotField(t, m, uibots.NameField, "operator")
	typeBotField(t, m, uibots.DirField, dir)
	typeBotField(t, m, uibots.SpecializationField, "fractory-operator")
	typeBotField(t, m, uibots.MemoryField, "fractory-ops")
	m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyEnter})
	m.drainEffects(t)
	if _, open := surfaceOf[*uibots.Surface](m); open {
		t.Fatalf("a stored Profile left the dialog open; status %q", m.errBar.text)
	}
	added := onlyBotProfile(t, m)
	tool := m.services.cfg.AgentToolNames()[0]
	if added.ID == "" || added.Name != "operator" || added.Tool != tool || added.Dir != dir || added.Specialization != "fractory-operator" || added.MemoryProject != "fractory-ops" {
		t.Fatalf("stored %+v", added)
	}

	p, _, _ := botBook{m.services.store}.BotProfile(added.ID)
	m.openBotDialog(&p)
	if view := ansi.Strip(preparedView(m)); !strings.Contains(view, "Edit Bot Profile") || !strings.Contains(view, "fractory-ops") {
		t.Fatalf("the edit does not open on what is stored:\n%s", view)
	}
	typeBotField(t, m, uibots.NameField, "watcher")
	m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyEnter})
	m.drainEffects(t)
	edited := onlyBotProfile(t, m)
	if edited.ID != added.ID || edited.Name != "watcher" || edited.MemoryProject != "fractory-ops" {
		t.Fatalf("edited %+v from %+v", edited, added)
	}
}

func TestBotDialogShowsARefusalAgainstItsField(t *testing.T) {
	m := buildModel(t)
	m.openBotDialog(nil)
	typeBotField(t, m, uibots.MemoryField, "fractory-ops")
	m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyEnter})
	m.drainEffects(t)
	d := botDialog(t, m).Dialog()
	if r := d.Refusal(); r.Field != uibots.NameField || d.Focus() != uibots.NameField {
		t.Fatalf("refusal %+v focus %d, want name", r, d.Focus())
	}
	if view := ansi.Strip(preparedView(m)); !strings.Contains(view, "name needs 1 to 60 characters") {
		t.Fatalf("the reason is not drawn:\n%s", view)
	}
	m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyEsc})
	if _, open := surfaceOf[*uibots.Surface](m); open {
		t.Fatal("esc left the dialog open")
	}

	gone := uibots.Profile{ID: "p1", Name: "operator", Tool: "retired-cli"}
	m.openBotDialog(&gone)
	m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyEnter})
	m.drainEffects(t)
	d = botDialog(t, m).Dialog()
	if r := d.Refusal(); r.Field != uibots.ToolField || !strings.Contains(r.Reason, "retired-cli") {
		t.Fatalf("refusal %+v, want the CLI refused", r)
	}
	if all, _ := m.services.store.BotProfiles(); len(all) != 0 {
		t.Fatalf("a refused Profile was stored: %+v", all)
	}
}

// The model list is what the CLI answered the spawn picker's discovery,
// and moves when a later answer does.
func TestBotDialogListsTheModelsDiscoveryReports(t *testing.T) {
	m := buildModel(t)
	m.services.configDir = t.TempDir()
	tool := m.services.cfg.Tools["claude"]
	tool.Catalog, tool.CatalogCommand, tool.ModelArgs = "claude", "sh", "--model {model}"
	m.services.cfg.Tools["claude"] = tool
	models := botsCatalog{m}
	if models.Load("claude") == nil {
		t.Fatal("the dialog's CLI was not asked for its models")
	}
	if got := models.Models("claude"); len(got.Keys) != 0 || !strings.Contains(got.Note, "reading models") {
		t.Fatalf("before any answer = %+v", got)
	}
	m.handleCatalog(uilaunch.CatalogMsg{Tool: "claude", Cat: claudeAnswer})
	if got := models.Models("claude").Keys; !slices.Equal(got, []string{"default", "opus", "haiku"}) {
		t.Fatalf("after the answer = %v", got)
	}
	m.handleCatalog(uilaunch.CatalogMsg{Tool: "claude", Cat: catalog.Catalog{Models: []catalog.Model{{ID: "opus-next"}}}})
	if got := models.Models("claude").Keys; !slices.Equal(got, []string{"opus-next"}) {
		t.Fatalf("after a later answer = %v", got)
	}
	p := uibots.Profile{ID: "p1", Name: "operator", Tool: "claude"}
	m.openBotDialog(&p)
	for botDialog(t, m).Dialog().Focus() != uibots.ModelField {
		m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyTab})
	}
	m.applyTestMsg(t, tea.KeyMsg{Type: tea.KeyRight})
	if view := ansi.Strip(preparedView(m)); !strings.Contains(view, "opus-next") {
		t.Fatalf("→ did not pick the discovered model:\n%s", view)
	}
}
