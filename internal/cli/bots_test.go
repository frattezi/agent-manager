package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/store"
	"github.com/YoanWai/agent-manager/internal/ui/bots"
)

// botSpecs is sampleSpecs with a Bot Profile's fields, as ui.SettingSpecs
// hands them over.
func botSpecs() ([]store.SettingSpec, error) {
	specs, _ := sampleSpecs()
	for _, spec := range bots.Specs([]string{"claude", "codex"}, 7) {
		specs = append(specs, store.SettingSpec{Key: spec.Key, Values: spec.Values, Default: spec.Default, Bot: true})
	}
	return specs, nil
}

var botNow = time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)

func createBot(t *testing.T, configDir string, args ...string) botRecord {
	t.Helper()
	var out bytes.Buffer
	if err := runBotsCreate(&out, botSpecs, append(args, "--json"), configDir, botNow); err != nil {
		t.Fatal(err)
	}
	var record botRecord
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("create --json %q: %v", out.String(), err)
	}
	return record
}

func showBot(t *testing.T, configDir, key string) botRecord {
	t.Helper()
	var out bytes.Buffer
	if err := runBotsShow(&out, botSpecs, []string{key}, configDir, botNow); err != nil {
		t.Fatal(err)
	}
	var record botRecord
	if err := json.Unmarshal(out.Bytes(), &record); err != nil {
		t.Fatalf("show %q: %v", out.String(), err)
	}
	return record
}

func TestBotsListIsEmptyOnAFreshStore(t *testing.T) {
	configDir := t.TempDir()
	var out bytes.Buffer
	if err := runBotsList(&out, botSpecs, nil, configDir, botNow); err != nil || out.String() != "" {
		t.Fatalf("list = %q, %v; want nothing", out.String(), err)
	}
	if err := runBotsList(&out, botSpecs, []string{"--json"}, configDir, botNow); err != nil || strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("list --json = %q, %v; want []", out.String(), err)
	}
}

func TestBotsCreateShowAndList(t *testing.T) {
	configDir := t.TempDir()
	dir := t.TempDir()
	created := createBot(t, configDir, "--name", "operator", "--emoji", "🛠", "--color", "3", "--tool", "claude",
		"--model", "opus", "--dir", dir, "--flags", "--permission-mode plan", "--specialization", "fractory-operator", "--memory", "fractory-ops")
	want := botRecord{
		ID: created.ID, Name: "operator", Emoji: "🛠", Color: 3, Tool: "claude", Model: "opus", Dir: dir,
		Flags: "--permission-mode plan", Specialization: "fractory-operator", MemoryProject: "fractory-ops",
		CreatedAt: botNow, UpdatedAt: botNow,
	}
	if created.ID == "" || !reflect.DeepEqual(created, want) {
		t.Fatalf("created %+v, want %+v", created, want)
	}
	if shown := showBot(t, configDir, "operator"); !reflect.DeepEqual(shown, want) {
		t.Fatalf("show by name = %+v, want %+v", shown, want)
	}
	if shown := showBot(t, configDir, created.ID); !reflect.DeepEqual(shown, want) {
		t.Fatalf("show by id = %+v, want %+v", shown, want)
	}

	var out bytes.Buffer
	if err := runBotsList(&out, botSpecs, nil, configDir, botNow); err != nil {
		t.Fatal(err)
	}
	if fields := strings.Fields(out.String()); !reflect.DeepEqual(fields, []string{created.ID, "🛠", "operator", "claude", "opus"}) {
		t.Fatalf("list = %q", out.String())
	}
	out.Reset()
	if err := runBotsList(&out, botSpecs, []string{"--json"}, configDir, botNow); err != nil {
		t.Fatal(err)
	}
	var listed []botRecord
	if err := json.Unmarshal(out.Bytes(), &listed); err != nil || len(listed) != 1 || !reflect.DeepEqual(listed[0], want) {
		t.Fatalf("list --json = %q, %v", out.String(), err)
	}
}

func TestBotsCreatePrintsASentenceAndResolvesTheDirectory(t *testing.T) {
	configDir := t.TempDir()
	t.Chdir(t.TempDir())
	var out bytes.Buffer
	if err := runBotsCreate(&out, botSpecs, []string{"--name", "ops", "--tool", "codex", "--dir", "work"}, configDir, botNow); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "created ops (") {
		t.Fatalf("create = %q", out.String())
	}
	shown := showBot(t, configDir, "ops")
	if !filepath.IsAbs(shown.Dir) || filepath.Base(shown.Dir) != "work" || shown.Color != 0 {
		t.Fatalf("stored %+v, want an absolute dir and the first swatch", shown)
	}
}

func TestBotsEditChangesOnlyTheFieldsGiven(t *testing.T) {
	configDir := t.TempDir()
	created := createBot(t, configDir, "--name", "operator", "--tool", "claude", "--model", "opus", "--flags", "--x")
	later := botNow.Add(time.Hour)
	var out bytes.Buffer
	if err := runBotsEdit(&out, botSpecs, []string{"operator", "--color", "5", "--model", "", "--json"}, configDir, later); err != nil {
		t.Fatal(err)
	}
	var edited botRecord
	if err := json.Unmarshal(out.Bytes(), &edited); err != nil {
		t.Fatal(err)
	}
	want := created
	want.Color, want.Model, want.UpdatedAt = 5, "", later
	if !reflect.DeepEqual(edited, want) {
		t.Fatalf("edited %+v, want %+v", edited, want)
	}
	if shown := showBot(t, configDir, created.ID); !reflect.DeepEqual(shown, want) {
		t.Fatalf("show after edit = %+v, want %+v", shown, want)
	}
	out.Reset()
	if err := runBotsEdit(&out, botSpecs, []string{created.ID, "--name", "watcher"}, configDir, later); err != nil || out.String() != "changed watcher ("+created.ID+")\n" {
		t.Fatalf("rename = %q, %v", out.String(), err)
	}
	if err := runBotsEdit(&out, botSpecs, []string{"watcher"}, configDir, later); err == nil || !strings.Contains(err.Error(), "nothing to change") {
		t.Fatalf("edit with no field = %v", err)
	}
}

// The shell refuses what the dialog would never write: each value is held
// to its SettingSpecs entry, and nothing is stored.
func TestBotsRefusesAValueOutsideItsSpec(t *testing.T) {
	configDir := t.TempDir()
	for _, args := range [][]string{
		{"--name", "ops", "--tool", "grok"},
		{"--name", "ops", "--tool", "claude", "--color", "7"},
		{"--name", "ops", "--tool", "claude", "--color", "#ff8800"},
		{"--name", "", "--tool", "claude"},
		{"--tool", "claude"},
		{"--name", "ops"},
	} {
		if err := runBotsCreate(&bytes.Buffer{}, botSpecs, args, configDir, botNow); err == nil {
			t.Errorf("create %v succeeded", args)
		}
	}
	var out bytes.Buffer
	if err := runBotsList(&out, botSpecs, nil, configDir, botNow); err != nil || out.String() != "" {
		t.Fatalf("a refused create stored %q, %v", out.String(), err)
	}

	created := createBot(t, configDir, "--name", "ops", "--tool", "claude")
	err := runBotsEdit(&bytes.Buffer{}, botSpecs, []string{"ops", "--tool", "grok"}, configDir, botNow)
	if err == nil || !strings.Contains(err.Error(), bots.ToolKey) {
		t.Fatalf("edit to grok = %v, want bot.tool named", err)
	}
	if shown := showBot(t, configDir, created.ID); shown.Tool != "claude" {
		t.Fatalf("a refused edit stored %+v", shown)
	}
}

func TestBotsShowAndEditNeedAProfileTheyCanFind(t *testing.T) {
	configDir := t.TempDir()
	first := createBot(t, configDir, "--name", "ops", "--tool", "claude")
	second := createBot(t, configDir, "--name", "ops", "--tool", "codex")
	err := runBotsShow(&bytes.Buffer{}, botSpecs, []string{"ops"}, configDir, botNow)
	if !errors.Is(err, bots.ErrAmbiguous) || !strings.Contains(err.Error(), first.ID) || !strings.Contains(err.Error(), second.ID) {
		t.Fatalf("show by a shared name = %v, want both ids named", err)
	}
	if err := runBotsEdit(&bytes.Buffer{}, botSpecs, []string{"nosuch", "--color", "1"}, configDir, botNow); err == nil {
		t.Fatal("edit of a missing Profile succeeded")
	}
}

// A Profile's fields are in SettingSpecs, but settings and export never
// see them: they live on the Profile's row.
func TestSettingsVerbsLeaveBotFieldsOut(t *testing.T) {
	table := Commands("dev", botSpecs)
	configDir := t.TempDir()
	if err := table["settings"]([]string{"set", bots.ColorKey, "1"}, nil, configDir); err == nil || !strings.Contains(err.Error(), "no setting is called") {
		t.Fatalf("settings set %s = %v, want it unknown", bots.ColorKey, err)
	}
	specs, err := sampleSpecs()
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := settingSpecs(botSpecs).settingsTable()()
	if err != nil || !reflect.DeepEqual(filtered, specs) {
		t.Fatalf("settings table = %+v, %v; want %+v", filtered, err, specs)
	}
}

func TestBotsGroupDispatchesEveryVerb(t *testing.T) {
	var names []string
	for _, verb := range botsSection(botSpecs).commands[0].verbs {
		names = append(names, verb.name)
	}
	if got := strings.Join(names, " "); got != "list show create edit" {
		t.Fatalf("verbs = %s", got)
	}
}
