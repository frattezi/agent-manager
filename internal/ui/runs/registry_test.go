package runs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeRun(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func record(id, status, updated, extra string) string {
	return `{"version":1,"id":"` + id + `","key":"k","ticketRef":"jira:ABC-1","repoPath":"/r","trigger":{"kind":"manual"},` +
		`"status":"` + status + `","branch":"b-` + id + `","logPath":"/tmp/` + id + `.log","createdAt":"2026-01-01T00:00:00Z",` +
		`"updatedAt":"` + updated + `","futureField":{"x":1}` + extra + `}`
}

func TestLoadMissingDirIsNoRuns(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "absent"))
	if err != nil || len(got) != 0 {
		t.Fatalf("Load = %v, %v; want no runs and no error", got, err)
	}
}

func TestLoadSkipsInvalidFilesAndIgnoresUnknownFields(t *testing.T) {
	dir := t.TempDir()
	writeRun(t, dir, "a.json", record("a", "done", "2026-01-02T00:00:00Z", ""))
	writeRun(t, dir, "bad-json.json", `{not json`)
	writeRun(t, dir, "v2.json", `{"version":2,"id":"v2","ticketRef":"jira:ABC-1","status":"done","logPath":"/l","updatedAt":"2026-01-02T00:00:00Z"}`)
	writeRun(t, dir, "status.json", record("s", "exploded", "2026-01-02T00:00:00Z", ""))
	writeRun(t, dir, "nolog.json", `{"version":1,"id":"n","ticketRef":"jira:ABC-1","status":"done","updatedAt":"2026-01-02T00:00:00Z"}`)
	writeRun(t, dir, "a.comments.json", `{"version":1,"comments":[]}`)
	writeRun(t, dir, "a.json.123.tmp", record("tmp", "done", "2026-01-02T00:00:00Z", ""))
	if err := os.Mkdir(filepath.Join(dir, "dir.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "a" || got[0].Branch != "b-a" {
		t.Fatalf("Load = %+v; want only run a", got)
	}
}

func TestLoadSortsActiveFirstThenNewestUpdate(t *testing.T) {
	dir := t.TempDir()
	writeRun(t, dir, "old-done.json", record("old-done", "done", "2026-01-01T00:00:00Z", ""))
	writeRun(t, dir, "new-done.json", record("new-done", "failed", "2026-01-05T00:00:00Z", ""))
	writeRun(t, dir, "queued.json", record("queued", "queued", "2026-01-02T00:00:00Z", ""))
	writeRun(t, dir, "running.json", record("running", "running", "2026-01-03T00:00:00Z", ""))
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range got {
		ids = append(ids, r.ID)
	}
	want := []string{"running", "queued", "new-done", "old-done"}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids = %v, want %v", ids, want)
		}
	}
}

func TestQuestionOnlyForNeedsHuman(t *testing.T) {
	dir := t.TempDir()
	result := `,"result":{"outcome":"needs-human","reason":"ambiguous-spec","question":"Which table?"}`
	writeRun(t, dir, "h.json", record("h", "needs-human", "2026-01-02T00:00:00Z", result))
	writeRun(t, dir, "d.json", record("d", "done", "2026-01-01T00:00:00Z", result))
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != "h" || got[0].Question() != "Which table?" {
		t.Fatalf("needs-human run = %+v", got[0])
	}
	if got[1].Question() != "" {
		t.Fatalf("done run shows a question: %q", got[1].Question())
	}
}

func TestDirHonoursFractoryHome(t *testing.T) {
	t.Setenv("FRACTORY_HOME", "/srv/fractory")
	if got := Dir(); got != "/srv/fractory/runs" {
		t.Fatalf("Dir = %q", got)
	}
	t.Setenv("FRACTORY_HOME", "")
	home, _ := os.UserHomeDir()
	if got := Dir(); got != filepath.Join(home, ".fractory", "runs") {
		t.Fatalf("Dir = %q", got)
	}
}

func TestBaseBranchReadsTheRepoConfigAndDefaultsToMain(t *testing.T) {
	repo := t.TempDir()
	if got := BaseBranch(repo); got != "main" {
		t.Fatalf("no config: %q", got)
	}
	writeRun(t, repo, ".fractory.json", `{"baseBranch":"develop","other":1}`)
	if got := BaseBranch(repo); got != "develop" {
		t.Fatalf("configured: %q", got)
	}
	writeRun(t, repo, ".fractory.json", `{broken`)
	if got := BaseBranch(repo); got != "main" {
		t.Fatalf("malformed: %q", got)
	}
	if got := BaseBranch(""); got != "main" {
		t.Fatalf("empty repo path: %q", got)
	}
}

func TestLoadFillsTheBaseFromTheRunsRepo(t *testing.T) {
	dir, repo := t.TempDir(), t.TempDir()
	writeRun(t, repo, ".fractory.json", `{"baseBranch":"develop"}`)
	writeRun(t, dir, "a.json", `{"version":1,"id":"a","ticketRef":"jira:ABC-1","status":"done","logPath":"/l","repoPath":"`+repo+`","updatedAt":"2026-01-02T00:00:00Z"}`)
	got, err := Load(dir)
	if err != nil || len(got) != 1 || got[0].Base != "develop" {
		t.Fatalf("Load = %+v, %v", got, err)
	}
}

func TestParseReadsCleanedAtAndLeavesItZeroWhenAbsentOrNull(t *testing.T) {
	cleaned, err := Parse([]byte(record("c", "done", "2026-01-02T00:00:00Z", `,"cleanedAt":"2026-01-03T04:05:06Z"`)))
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 1, 3, 4, 5, 6, 0, time.UTC); !cleaned.CleanedAt.Equal(want) {
		t.Fatalf("cleanedAt = %v, want %v", cleaned.CleanedAt, want)
	}
	for _, extra := range []string{"", `,"cleanedAt":null`} {
		open, err := Parse([]byte(record("o", "done", "2026-01-02T00:00:00Z", extra)))
		if err != nil || !open.CleanedAt.IsZero() {
			t.Fatalf("extra %q: cleanedAt = %v, err %v; want zero", extra, open.CleanedAt, err)
		}
	}
}

func TestParseDecodesSummaryPRStateFinishedAtAndAgents(t *testing.T) {
	full, err := Parse([]byte(record("a", "done", "2026-01-02T00:00:00Z",
		`,"prState":"MERGED","finishedAt":"2026-01-02T00:30:00Z","agents":["claude@work"],"result":{"outcome":"done","summary":"Added the thing."}`)))
	if err != nil {
		t.Fatal(err)
	}
	if full.PRState != "MERGED" || full.FinishedAt.Format(time.RFC3339) != "2026-01-02T00:30:00Z" || full.Result.Summary != "Added the thing." || len(full.Agents) != 1 {
		t.Errorf("record = %+v", full)
	}
	old, err := Parse([]byte(record("b", "done", "2026-01-02T00:00:00Z", "")))
	if err != nil {
		t.Fatalf("an older record without the four fields does not parse: %v", err)
	}
	if old.PRState != "" || !old.FinishedAt.IsZero() || old.Result != nil || old.Agents != nil {
		t.Errorf("old record = %+v; want the four fields absent", old)
	}
}
