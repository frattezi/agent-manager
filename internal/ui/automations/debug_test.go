package automations

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func debugFixture(t *testing.T) (Config, Record, string) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "firing.log")
	if err := os.WriteFile(logPath, []byte("SECRET-LOG-LINE: reconcile failed on row 42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := Config{Version: Version, ID: "recon", Name: "Weekly reconcile", Kind: KindAgent, RepoPath: "/src/books", Cron: "0 7 * * 1",
		Timezone: "Europe/Lisbon", Mode: ModeWork, Tool: "claude", Entry: "prompts/reconcile.md", Retention: 3}
	record := Record{Version: RecordVersion, ID: "run-1", AutomationID: "recon", FiredFor: time.Date(2026, 7, 6, 6, 0, 0, 0, time.UTC),
		Status: "done", LogPath: logPath, WorktreePath: "/wt/recon-run-1", Result: &Result{Outcome: "failed", Summary: "SUMMARY-TEXT from the run"}}
	return config, record, logPath
}

func TestDebugSessionNamesTheFiringAndCarriesPathsNotContents(t *testing.T) {
	config, record, logPath := debugFixture(t)
	debug, err := DebugSession(config, record)
	if err != nil {
		t.Fatal(err)
	}
	if debug.Name != "debug-Weekly-reconcile-20260706-0700" {
		t.Fatalf("name = %q", debug.Name)
	}
	if debug.Dir != "/src/books" {
		t.Fatalf("dir = %q", debug.Dir)
	}
	for _, want := range []string{`"Weekly reconcile"`, "recon", "2026-07-06T07:00:00+01:00", "failed (status done)", `"` + logPath + `"`, `"/wt/recon-run-1"`, "prompts/reconcile.md"} {
		if !strings.Contains(debug.Prompt, want) {
			t.Fatalf("prompt lacks %q:\n%s", want, debug.Prompt)
		}
	}
	for _, pasted := range []string{"SECRET-LOG-LINE", "row 42", "SUMMARY-TEXT"} {
		if strings.Contains(debug.Prompt, pasted) {
			t.Fatalf("prompt pastes %q from the firing:\n%s", pasted, debug.Prompt)
		}
	}
}

func TestDebugSessionSaysWhatTheRecordLacks(t *testing.T) {
	config, record, _ := debugFixture(t)
	record.LogPath, record.WorktreePath, record.Result = "", "", nil
	record.Status = "failed"
	debug, err := DebugSession(config, record)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"outcome was failed.", "names no log", "names no worktree"} {
		if !strings.Contains(debug.Prompt, want) {
			t.Fatalf("prompt lacks %q:\n%s", want, debug.Prompt)
		}
	}
}

func TestDebugSessionRefusesAnUnfinishedFiring(t *testing.T) {
	config, record, _ := debugFixture(t)
	for _, status := range []string{StatusQueued, StatusRunning} {
		record.Status = status
		if _, err := DebugSession(config, record); !errors.Is(err, ErrUnfinished) {
			t.Fatalf("%s firing: err = %v; want ErrUnfinished", status, err)
		}
	}
}

func TestProjectGroupFindsTheRepositorysGroupOrNamesANewOne(t *testing.T) {
	groups := []Group{
		{Path: "work/books", Dir: "/src/books"},
		{Path: "books", Dir: "/src/books/"},
		{Path: "old", Dir: "/src/books", Archived: true},
		{Path: "ledger", Dir: "/src/other"},
	}
	if group, create := ProjectGroup("/src/books", groups); group != "books" || create {
		t.Fatalf("existing = %q, %v; want the shallowest live group", group, create)
	}
	if group, create := ProjectGroup("/src/ledger", groups); group != "ledger-2" || !create {
		t.Fatalf("new = %q, %v; want a fresh name beside the taken one", group, create)
	}
	if group, create := ProjectGroup("/src/books", groups[2:3]); group != "books" || !create {
		t.Fatalf("only an archived group = %q, %v; want a new live one", group, create)
	}
}
