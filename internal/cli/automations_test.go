package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/ui/automations"
)

// automationDoc is a valid configuration whose directory is a Fractory
// repository made for the test.
func automationDoc(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".fractory.json"), []byte(`{"mode":"work"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return automationDocAt(repo)
}

func automationDocAt(repo string) string {
	path, _ := json.Marshal(repo)
	return `{"name":"weekly","kind":"agent","repoPath":` + string(path) + `,"cron":"0 7 * * 1",
"timezone":"Europe/Lisbon","mode":"work","tool":"claude","flags":[],"entry":"prompts/weekly.md","retention":5}`
}

var automationNow = time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)

func addAutomation(t *testing.T, configDir, doc string, args ...string) automationRecord {
	t.Helper()
	var out bytes.Buffer
	if err := runAutomationsAdd(&out, strings.NewReader(doc), append(args, "--json"), configDir, automationNow); err != nil {
		t.Fatal(err)
	}
	var record automationRecord
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("add --json %q: %v", out.String(), err)
	}
	return record
}

func TestAutomationsListIsEmptyOnAFreshStore(t *testing.T) {
	configDir := t.TempDir()
	var out bytes.Buffer
	if err := runAutomationsList(&out, nil, nil, configDir, automationNow); err != nil {
		t.Fatal(err)
	}
	if out.String() != "" {
		t.Fatalf("list = %q, want nothing", out.String())
	}
	out.Reset()
	if err := runAutomationsList(&out, nil, []string{"--json"}, configDir, automationNow); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("list --json = %q, want []", out.String())
	}
}

func TestAutomationsAddShowListAndRemove(t *testing.T) {
	configDir := t.TempDir()
	added := addAutomation(t, configDir, automationDoc(t))
	if added.ID == "" || added.Name != "weekly" || !added.Enabled || added.LastFiredFor != nil || !added.CreatedAt.Equal(automationNow) {
		t.Fatalf("added = %+v", added)
	}
	var stored automations.Config
	if err := json.Unmarshal(added.Config, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.ID != added.ID || stored.Version != 1 || stored.Entry != "prompts/weekly.md" {
		t.Fatalf("stored configuration = %+v, want the assigned id and version 1 inside it", stored)
	}

	var out bytes.Buffer
	if err := runAutomationsShow(&out, nil, []string{"weekly"}, configDir, automationNow); err != nil {
		t.Fatal(err)
	}
	var shown automationRecord
	if err := json.Unmarshal(out.Bytes(), &shown); err != nil || shown.ID != added.ID || !bytes.Equal(compactJSON(t, shown.Config), compactJSON(t, added.Config)) {
		t.Fatalf("show by name = %s, %v", out.String(), err)
	}

	out.Reset()
	if err := runAutomationsList(&out, nil, nil, configDir, automationNow); err != nil {
		t.Fatal(err)
	}
	if want := added.ID + "  weekly  enabled  0 7 * * 1\n"; out.String() != want {
		t.Fatalf("list = %q, want %q", out.String(), want)
	}

	out.Reset()
	if err := runAutomationsRemove(&out, nil, []string{added.ID}, configDir, automationNow); err != nil {
		t.Fatal(err)
	}
	if out.String() != "removed weekly ("+added.ID+")\n" {
		t.Fatalf("remove = %q", out.String())
	}
	if err := runAutomationsShow(&out, nil, []string{added.ID}, configDir, automationNow); err == nil || !strings.Contains(err.Error(), "no Automation") {
		t.Fatalf("show after remove = %v", err)
	}
	if err := runAutomationsRemove(&out, nil, []string{added.ID}, configDir, automationNow); err == nil {
		t.Fatal("second remove succeeded")
	}
}

func TestAutomationsAddReadsAFileAndCanStartDisabled(t *testing.T) {
	configDir := t.TempDir()
	path := filepath.Join(t.TempDir(), "weekly.json")
	if err := os.WriteFile(path, []byte(automationDoc(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runAutomationsAdd(&out, strings.NewReader("ignored"), []string{"--disabled", path}, configDir, automationNow); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "added weekly (") {
		t.Fatalf("add = %q", out.String())
	}
	all, err := settingsStore(t, configDir).Automations()
	if err != nil || len(all) != 1 || all[0].Enabled {
		t.Fatalf("stored = %+v, %v; want one disabled Automation", all, err)
	}
}

func TestAutomationsChangeKeepsTheIDAndState(t *testing.T) {
	configDir := t.TempDir()
	added := addAutomation(t, configDir, automationDoc(t))
	if err := settingsStore(t, configDir).SetAutomationLastFired(added.ID, automationNow); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runAutomationsDisable(&out, nil, []string{added.ID}, configDir, automationNow); err != nil {
		t.Fatal(err)
	}
	renamed := strings.Replace(automationDoc(t), `"weekly"`, `"monday sweep"`, 1)
	later := automationNow.Add(time.Hour)
	out.Reset()
	if err := runAutomationsChange(&out, strings.NewReader(renamed), []string{"weekly", "--json"}, configDir, later); err != nil {
		t.Fatal(err)
	}
	var changed automationRecord
	if err := json.Unmarshal(out.Bytes(), &changed); err != nil {
		t.Fatal(err)
	}
	var config automations.Config
	_ = json.Unmarshal(changed.Config, &config)
	if changed.ID != added.ID || config.ID != added.ID || changed.Name != "monday sweep" || config.Name != "monday sweep" {
		t.Fatalf("changed = %+v / %+v, want the id %s kept", changed, config, added.ID)
	}
	if changed.Enabled || changed.LastFiredFor == nil || !changed.LastFiredFor.Equal(automationNow) || !changed.UpdatedAt.Equal(later) || !changed.CreatedAt.Equal(automationNow) {
		t.Fatalf("change touched the state: %+v", changed)
	}

	withOtherID := strings.Replace(automationDoc(t), `{`, `{"id":"other",`, 1)
	err := runAutomationsChange(&out, strings.NewReader(withOtherID), []string{added.ID}, configDir, later)
	if err == nil || !strings.HasPrefix(err.Error(), "id: ") {
		t.Fatalf("change to another id = %v, want a refusal naming id", err)
	}
	withSameID := strings.Replace(automationDoc(t), `{`, `{"id":"`+added.ID+`",`, 1)
	if err := runAutomationsChange(&out, strings.NewReader(withSameID), []string{added.ID}, configDir, later); err != nil {
		t.Fatalf("change naming its own id = %v", err)
	}
}

func TestAutomationsEnableAndDisable(t *testing.T) {
	configDir := t.TempDir()
	added := addAutomation(t, configDir, automationDoc(t), "--disabled")
	var out bytes.Buffer
	if err := runAutomationsEnable(&out, nil, []string{added.ID}, configDir, automationNow); err != nil {
		t.Fatal(err)
	}
	if out.String() != "enabled weekly ("+added.ID+")\n" {
		t.Fatalf("enable = %q", out.String())
	}
	if a, _, _ := settingsStore(t, configDir).Automation(added.ID); !a.Enabled {
		t.Fatal("enable left it disabled")
	}
	out.Reset()
	if err := runAutomationsDisable(&out, nil, []string{"weekly"}, configDir, automationNow); err != nil {
		t.Fatal(err)
	}
	if out.String() != "disabled weekly ("+added.ID+")\n" {
		t.Fatalf("disable = %q", out.String())
	}
	a, _, _ := settingsStore(t, configDir).Automation(added.ID)
	if a.Enabled || !bytes.Equal(a.Config, compactJSON(t, added.Config)) {
		t.Fatalf("after disable = %+v, want the configuration untouched", a)
	}
	if err := runAutomationsEnable(&out, nil, []string{"missing"}, configDir, automationNow); err == nil {
		t.Fatal("enable of a missing Automation succeeded")
	}
}

func TestAutomationsRefusesAMalformedConfigurationNamingTheField(t *testing.T) {
	configDir := t.TempDir()
	var out bytes.Buffer
	bad := strings.Replace(automationDoc(t), `"cron":"0 7 * * 1"`, `"cron":"every monday"`, 1)
	err := runAutomationsAdd(&out, strings.NewReader(bad), nil, configDir, automationNow)
	if err == nil || !strings.HasPrefix(err.Error(), "cron: ") {
		t.Fatalf("add = %v, want a refusal naming cron", err)
	}
	unknownTool := strings.Replace(automationDoc(t), `"tool":"claude"`, `"tool":"not-a-cli"`, 1)
	if err := runAutomationsAdd(&out, strings.NewReader(unknownTool), nil, configDir, automationNow); err == nil || !strings.HasPrefix(err.Error(), "tool: ") {
		t.Fatalf("add = %v, want a refusal naming tool", err)
	}
	if err := runAutomationsAdd(&out, strings.NewReader(automationDocAt(t.TempDir())), nil, configDir, automationNow); err == nil ||
		!strings.HasPrefix(err.Error(), "repoPath: ") || !strings.Contains(err.Error(), "not a Fractory repository") {
		t.Fatalf("add = %v, want a refusal naming repoPath", err)
	}
	if all, _ := settingsStore(t, configDir).Automations(); len(all) != 0 {
		t.Fatalf("a refused configuration was stored: %+v", all)
	}

	added := addAutomation(t, configDir, automationDoc(t))
	noEntry := strings.Replace(automationDoc(t), `"entry":"prompts/weekly.md"`, `"entry":""`, 1)
	if err := runAutomationsChange(&out, strings.NewReader(noEntry), []string{added.ID}, configDir, automationNow); err == nil || !strings.HasPrefix(err.Error(), "entry: ") {
		t.Fatalf("change = %v, want a refusal naming entry", err)
	}
	if a, _, _ := settingsStore(t, configDir).Automation(added.ID); !bytes.Equal(a.Config, compactJSON(t, added.Config)) {
		t.Fatalf("a refused change was stored: %s", a.Config)
	}
}

func TestAutomationsRefusesASharedID(t *testing.T) {
	configDir := t.TempDir()
	doc := strings.Replace(automationDoc(t), `{`, `{"id":"fixed",`, 1)
	addAutomation(t, configDir, doc)
	var out bytes.Buffer
	err := runAutomationsAdd(&out, strings.NewReader(doc), nil, configDir, automationNow)
	if err == nil || !strings.HasPrefix(err.Error(), "id: ") {
		t.Fatalf("second add = %v, want a refusal naming id", err)
	}
}

func TestAutomationsNameThatTwoCarryIsAmbiguous(t *testing.T) {
	configDir := t.TempDir()
	first := addAutomation(t, configDir, automationDoc(t))
	second := addAutomation(t, configDir, automationDoc(t))
	var out bytes.Buffer
	err := runAutomationsShow(&out, nil, []string{"weekly"}, configDir, automationNow)
	if err == nil || !strings.Contains(err.Error(), first.ID) || !strings.Contains(err.Error(), second.ID) {
		t.Fatalf("show by a shared name = %v, want both ids named", err)
	}
}

func TestAutomationsGroupDispatchesEveryVerb(t *testing.T) {
	var names []string
	for _, verb := range automationsSection().commands[0].verbs {
		names = append(names, verb.name)
	}
	if got := strings.Join(names, " "); got != "list show add change remove enable disable" {
		t.Fatalf("verbs = %s", got)
	}
}

func compactJSON(t *testing.T, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := json.Compact(&out, data); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
