package automations

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/YoanWai/agent-manager/internal/ui/runs"
)

func writeFile(t *testing.T, dir, name, data string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func recordJSON(id, firedFor, updatedAt string) string {
	return `{"version":1,"id":"` + id + `","automationId":"weekly-1","firedFor":"` + firedFor + `","status":"done",` +
		`"startedAt":"` + firedFor + `","updatedAt":"` + updatedAt + `","logPath":"/logs/` + id + `.log",` +
		`"result":{"outcome":"done","summary":"ok"},"outputPath":"/out/` + id + `"}`
}

func automationDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "weekly-1")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadRecordsLeavesRequestsAndRefusedAlone(t *testing.T) {
	dir := automationDir(t)
	root := filepath.Dir(dir)
	writeFile(t, dir, "a.json", recordJSON("a", "2026-11-26T07:00:00Z", "2026-11-26T07:05:00Z"))
	for _, name := range []string{"requests", "refused"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(root, name), "x.json", `not a record`)
	}
	got, skips, err := LoadRecords(root)
	if err != nil || len(skips) != 0 || len(got) != 1 {
		t.Fatalf("LoadRecords = %+v, %+v, %v", got, skips, err)
	}
}

func TestLoadRecordsReturnsEveryValidRecordNewestFirst(t *testing.T) {
	dir := automationDir(t)
	writeFile(t, dir, "a.json", recordJSON("a", "2026-11-24T07:00:00Z", "2026-11-24T07:05:00Z"))
	writeFile(t, dir, "c.json", recordJSON("c", "2026-11-26T07:00:00Z", "2026-11-26T07:05:00Z"))
	writeFile(t, dir, "b.json", recordJSON("b", "2026-11-25T07:00:00Z", "2026-11-27T00:00:00Z"))
	got, skips, err := LoadRecords(filepath.Dir(dir))
	if err != nil || len(skips) != 0 {
		t.Fatalf("LoadRecords = %v, %v", skips, err)
	}
	var ids []string
	for _, r := range got {
		ids = append(ids, r.ID)
	}
	if strings.Join(ids, ",") != "c,b,a" {
		t.Fatalf("order = %v, want c,b,a", ids)
	}
	c := got[0]
	if c.AutomationID != "weekly-1" || c.Status != "done" || c.LogPath != "/logs/c.log" || c.OutputPath != "/out/c" ||
		c.Result == nil || c.Result.Outcome != "done" || c.StartedAt.IsZero() || c.UpdatedAt.IsZero() {
		t.Fatalf("record = %+v", c)
	}
}

func TestLoadRecordsSkipsAnotherVersionWithAReason(t *testing.T) {
	dir := automationDir(t)
	writeFile(t, dir, "ok.json", recordJSON("ok", "2026-11-26T07:00:00Z", "2026-11-26T07:05:00Z"))
	writeFile(t, dir, "v2.json", strings.Replace(recordJSON("v2", "2026-11-26T07:00:00Z", "2026-11-26T07:05:00Z"), `"version":1`, `"version":2`, 1))
	got, skips, err := LoadRecords(filepath.Dir(dir))
	if err != nil || len(got) != 1 || got[0].ID != "ok" {
		t.Fatalf("LoadRecords = %+v, %v", got, err)
	}
	if len(skips) != 1 || filepath.Base(skips[0].Path) != "v2.json" || !strings.Contains(skips[0].Reason.Error(), "version 2") {
		t.Fatalf("skips = %+v", skips)
	}
}

func TestLoadRecordsIgnoresAnUnknownField(t *testing.T) {
	dir := automationDir(t)
	writeFile(t, dir, "a.json", strings.Replace(recordJSON("a", "2026-11-26T07:00:00Z", "2026-11-26T07:05:00Z"), `{"version":1`, `{"version":1,"costUsd":0.42,"extra":{"x":[1]}`, 1))
	got, skips, err := LoadRecords(filepath.Dir(dir))
	if err != nil || len(skips) != 0 || len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("LoadRecords = %+v, %+v, %v", got, skips, err)
	}
}

func TestLoadRecordsSkipsAFileItCannotUse(t *testing.T) {
	dir := automationDir(t)
	writeFile(t, dir, "ok.json", recordJSON("ok", "2026-11-26T07:00:00Z", "2026-11-26T07:05:00Z"))
	writeFile(t, dir, "malformed.json", `{"version":1,`)
	writeFile(t, dir, "missing.json", `{"version":1,"id":"m"}`)
	writeFile(t, dir, ".ok.json-123", `partial`)
	writeFile(t, dir, "notes.txt", `not a record`)
	if err := os.Mkdir(filepath.Join(dir, "dir.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	want := 2
	if runtime.GOOS != "windows" && os.Geteuid() != 0 {
		writeFile(t, dir, "unreadable.json", recordJSON("u", "2026-11-26T07:00:00Z", "2026-11-26T07:05:00Z"))
		if err := os.Chmod(filepath.Join(dir, "unreadable.json"), 0); err != nil {
			t.Fatal(err)
		}
		want++
	}
	got, skips, err := LoadRecords(filepath.Dir(dir))
	if err != nil || len(got) != 1 || got[0].ID != "ok" {
		t.Fatalf("LoadRecords = %+v, %v", got, err)
	}
	if len(skips) != want {
		t.Fatalf("skips = %+v, want %d", skips, want)
	}
	for _, s := range skips {
		if s.Reason == nil {
			t.Fatalf("skip %s has no reason", s.Path)
		}
	}
}

func TestAnAbsentRecordsDirectoryIsNoRecords(t *testing.T) {
	got, skips, err := LoadRecords(filepath.Join(t.TempDir(), "absent"))
	if err != nil || len(got) != 0 || len(skips) != 0 {
		t.Fatalf("LoadRecords = %v, %v, %v", got, skips, err)
	}
}

func TestDirectoriesResolveBesideTheRunsRegistry(t *testing.T) {
	t.Setenv("FRACTORY_HOME", "/srv/fractory")
	home := filepath.Dir(runs.Dir())
	if RequestsDir() != filepath.Join(home, "automations", "requests") || RecordsDir() != filepath.Join(home, "automations") {
		t.Fatalf("dirs = %q, %q beside %q", RequestsDir(), RecordsDir(), home)
	}
	fallback := t.TempDir()
	t.Setenv("FRACTORY_HOME", "")
	t.Setenv("HOME", fallback)
	t.Setenv("USERPROFILE", fallback)
	home = filepath.Dir(runs.Dir())
	if home != filepath.Join(fallback, ".fractory") || RecordsDir() != filepath.Join(home, "automations") || RequestsDir() != filepath.Join(home, "automations", "requests") {
		t.Fatalf("fallback dirs = %q, %q beside %q", RequestsDir(), RecordsDir(), home)
	}
}
