package automations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const stored = `{"version":1,"id":"weekly-1","name":"weekly","kind":"agent","repoPath":"/src/repo","cron":"0 7 * * MON-FRI","timezone":"Europe/Lisbon","mode":"work","cli":"claude","flags":["--yes"],"entryPoint":"prompts/weekly.md","keepRuns":5}`

var firedFor = time.Date(2026, 11, 26, 7, 0, 0, 0, time.FixedZone("Lisbon", 0))

func TestWriteRequestWritesExactlyTheContractOnce(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 26, 4, 0, 30, 500, time.FixedZone("x", -3*3600))
	wrote, err := WriteRequest(dir, []byte(stored), firedFor, now)
	if err != nil || !wrote {
		t.Fatalf("first write = %v, %v; want written", wrote, err)
	}
	path := filepath.Join(dir, "weekly-1@20261126T070000Z.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"version":1,"requestedAt":"2026-11-26T07:00:30Z","firedFor":"2026-11-26T07:00:00Z","automation":` + stored + "}\n"
	if string(data) != want {
		t.Fatalf("file = %q\nwant   %q", data, want)
	}

	wrote, err = WriteRequest(dir, []byte(strings.Replace(stored, `"weekly"`, `"renamed"`, 1)), firedFor, now.Add(time.Hour))
	if err != nil || wrote {
		t.Fatalf("second write = %v, %v; want a no-op", wrote, err)
	}
	if again, _ := os.ReadFile(path); string(again) != want {
		t.Fatalf("second write changed the file: %q", again)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("dir holds %d files, want only the request (no temp files)", len(entries))
	}
}

func TestWriteRequestCreatesAnAbsentDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "automations")
	if wrote, err := WriteRequest(dir, []byte(stored), firedFor, firedFor); err != nil || !wrote {
		t.Fatalf("write = %v, %v", wrote, err)
	}
}

func TestRequestNameIsStableAcrossZones(t *testing.T) {
	a, err := RequestName("weekly-1", firedFor)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := RequestName("weekly-1", firedFor.In(time.FixedZone("y", 5*3600)))
	if a != b || a != "weekly-1@20261126T070000Z.json" {
		t.Fatalf("names = %q, %q", a, b)
	}
}

func TestWriteRequestRefusesAnUnsafeName(t *testing.T) {
	for _, tc := range []struct {
		name     string
		id       string
		firedFor time.Time
	}{
		{"empty id", "", firedFor},
		{"separator", "a/b", firedFor},
		{"backslash", `a\b`, firedFor},
		{"parent", "..", firedFor},
		{"hidden", ".weekly", firedFor},
		{"space", "my chore", firedFor},
		{"zero instant", "weekly-1", time.Time{}},
		{"sub-second instant", "weekly-1", firedFor.Add(time.Millisecond)},
		{"year past 9999", "weekly-1", time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			config := strings.Replace(stored, `"id":"weekly-1"`, `"id":`+quote(tc.id), 1)
			if wrote, err := WriteRequest(dir, []byte(config), tc.firedFor, firedFor); err == nil || wrote {
				t.Fatalf("write = %v, %v; want refused", wrote, err)
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 0 {
				t.Fatalf("a refused request left %d files", len(entries))
			}
		})
	}
}

func quote(s string) string { return `"` + strings.ReplaceAll(s, `\`, `\\`) + `"` }

func TestWriteRequestRefusesAConfigurationThatIsNotJSON(t *testing.T) {
	if _, err := WriteRequest(t.TempDir(), []byte(`{"id":`), firedFor, firedFor); err == nil {
		t.Fatal("a malformed configuration was accepted")
	}
}

func TestLoadRequestsListsWrittenRequestsAndSkipsTheRest(t *testing.T) {
	dir := t.TempDir()
	later := firedFor.Add(24 * time.Hour)
	for _, at := range []time.Time{later, firedFor} {
		if _, err := WriteRequest(dir, []byte(stored), at, at); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, dir, "v2@x.json", `{"version":2,"firedFor":"2026-11-26T07:00:00Z","automation":{}}`)
	writeFile(t, dir, "broken.json", `{`)
	got, skips, err := LoadRequests(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].FiredFor.Equal(firedFor) || !got[1].FiredFor.Equal(later) || string(got[0].Automation) != stored {
		t.Fatalf("LoadRequests = %+v", got)
	}
	if len(skips) != 2 {
		t.Fatalf("skips = %+v, want the v2 and the malformed file", skips)
	}
}

func TestAnAbsentRequestsDirectoryIsNoRequests(t *testing.T) {
	got, skips, err := LoadRequests(filepath.Join(t.TempDir(), "absent"))
	if err != nil || len(got) != 0 || len(skips) != 0 {
		t.Fatalf("LoadRequests = %v, %v, %v", got, skips, err)
	}
}
