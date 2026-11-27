package automations

import (
	"encoding/json"
	"errors"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
)

var tools = []string{"claude", "codex"}

const valid = `{"name":"weekly","kind":"agent","repoPath":"/src/repo","cron":"0 7 * * MON-FRI",
"timezone":"Europe/Lisbon","mode":"work","cli":"claude","flags":["--yes"],"entryPoint":"prompts/weekly.md","keepRuns":5}`

func TestParseFillsTheVersionAndKeepsEveryField(t *testing.T) {
	got, err := Parse([]byte(valid), tools)
	if err != nil {
		t.Fatal(err)
	}
	want := Config{Version: 1, Name: "weekly", Kind: "agent", RepoPath: "/src/repo", Cron: "0 7 * * MON-FRI",
		Timezone: "Europe/Lisbon", Mode: "work", Tool: "claude", Flags: []string{"--yes"}, Entry: "prompts/weekly.md", Retention: 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse = %+v, want %+v", got, want)
	}
	encoded, err := got.Encode()
	if err != nil {
		t.Fatal(err)
	}
	again, err := Parse(encoded, tools)
	if err != nil || !reflect.DeepEqual(again, want) {
		t.Fatalf("round trip = %+v, %v", again, err)
	}
}

func TestScriptAutomationNeedsNoTool(t *testing.T) {
	doc := strings.Replace(strings.Replace(valid, `"kind":"agent"`, `"kind":"script"`, 1), `"cli":"claude",`, ``, 1)
	got, err := Parse([]byte(doc), tools)
	if err != nil {
		t.Fatal(err)
	}
	if got.Flags == nil || got.Tool != "" {
		t.Fatalf("script = %+v", got)
	}
}

func TestParseRefusesNamingTheField(t *testing.T) {
	cases := []struct{ from, to, field string }{
		{`"name":"weekly"`, `"name":" "`, "name"},
		{`"kind":"agent"`, `"kind":"chat"`, "kind"},
		{`"repoPath":"/src/repo"`, `"repoPath":"src/repo"`, "repoPath"},
		{`"repoPath":"/src/repo"`, `"repoPath":""`, "repoPath"},
		{`"cron":"0 7 * * MON-FRI"`, `"cron":"0 7 * *"`, "cron"},
		{`"cron":"0 7 * * MON-FRI"`, `"cron":"61 7 * * *"`, "cron"},
		{`"cron":"0 7 * * MON-FRI"`, `"cron":"0 9-7 * * *"`, "cron"},
		{`"cron":"0 7 * * MON-FRI"`, `"cron":"*/0 * * * *"`, "cron"},
		{`"cron":"0 7 * * MON-FRI"`, `"cron":"0 7 * FOO *"`, "cron"},
		{`"timezone":"Europe/Lisbon"`, `"timezone":"Mars/Olympus"`, "timezone"},
		{`"timezone":"Europe/Lisbon"`, `"timezone":"Local"`, "timezone"},
		{`"timezone":"Europe/Lisbon"`, `"timezone":""`, "timezone"},
		{`"mode":"work"`, `"mode":"play"`, "mode"},
		{`"cli":"claude"`, `"cli":"vim"`, "cli"},
		{`"cli":"claude"`, `"cli":""`, "cli"},
		{`"entryPoint":"prompts/weekly.md"`, `"entryPoint":""`, "entryPoint"},
		{`"keepRuns":5`, `"keepRuns":0`, "keepRuns"},
		{`"keepRuns":5`, `"keepRuns":"5"`, "keepRuns"},
		{`"flags":["--yes"]`, `"flags":"--yes"`, "flags"},
		{`"name":"weekly"`, `"name":"weekly","version":2`, "version"},
		{`"name":"weekly"`, `"name":"weekly","shedule":"x"`, "shedule"},
	}
	for _, c := range cases {
		doc := strings.Replace(valid, c.from, c.to, 1)
		_, err := Parse([]byte(doc), tools)
		var field *FieldError
		if !errors.As(err, &field) || field.Field != c.field {
			t.Errorf("%s: err = %v, want a refusal naming %s", c.to, err, c.field)
			continue
		}
		if !strings.HasPrefix(err.Error(), c.field+": ") {
			t.Errorf("%s: message %q does not start with the field", c.to, err)
		}
	}
}

func TestParseRefusesWhatIsNotOneDocument(t *testing.T) {
	for _, doc := range []string{``, `not json`, valid + valid} {
		if _, err := Parse([]byte(doc), tools); err == nil {
			t.Errorf("Parse(%q) accepted", doc)
		}
	}
}

func TestValidateCronAcceptsTheUsualShapes(t *testing.T) {
	for _, expr := range []string{"* * * * *", "*/15 * * * *", "0 7 * * 1-5", "0 0 1,15 * *", "30 6 * jan-mar sun", "0 0 * * 7", "0-30/5 8-18 * * *"} {
		if err := ValidateCron(expr); err != nil {
			t.Errorf("ValidateCron(%q) = %v", expr, err)
		}
	}
}

// contractFields are the field names of version 1 of Fractory's published
// configuration contract, which its reader requires in every request.
var contractFields = []string{"version", "id", "name", "kind", "repoPath", "cron", "timezone", "mode", "cli", "flags", "entryPoint", "keepRuns"}

func TestEncodeWritesExactlyTheContractFieldNames(t *testing.T) {
	encoded, err := Config{Version: Version, ID: "weekly-1", Name: "weekly", Kind: KindScript, RepoPath: "/src/repo",
		Cron: "0 7 * * *", Timezone: "UTC", Mode: ModeWork, Entry: "scripts/weekly.sh", Retention: 3}.Encode()
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	got := slices.Sorted(maps.Keys(fields))
	want := slices.Sorted(slices.Values(contractFields))
	if !slices.Equal(got, want) {
		t.Fatalf("encoded fields = %v, want %v", got, want)
	}
}

func TestParseRefusesTheOldFieldNames(t *testing.T) {
	for _, old := range []struct{ from, to string }{
		{`"cli":`, `"tool":`}, {`"entryPoint":`, `"entry":`}, {`"keepRuns":`, `"retention":`},
	} {
		doc := strings.Replace(valid, old.from, old.to, 1)
		var field *FieldError
		if _, err := Parse([]byte(doc), tools); !errors.As(err, &field) || field.Field != strings.Trim(old.to, `":`) {
			t.Errorf("Parse with %s = %v, want a refusal of that field", old.to, err)
		}
	}
}
