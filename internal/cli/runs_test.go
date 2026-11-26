package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/ui/runs"
)

func writeRunRecord(t *testing.T, dir, id, ticket, created string) {
	t.Helper()
	record := `{"version":1,"id":"` + id + `","key":"` + ticket + `","ticketRef":"` + ticket + `","status":"failed","logPath":"/tmp/` + id + `.log","createdAt":"` + created + `","updatedAt":"` + created + `"}`
	if err := os.WriteFile(filepath.Join(dir, id+".json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunsResolveMarksTheTicketsLatestRunByIdOrRef(t *testing.T) {
	dir := t.TempDir()
	writeRunRecord(t, dir, "old", "github:o/r#1", "2026-01-01T00:00:00Z")
	writeRunRecord(t, dir, "new", "github:o/r#1", "2026-01-02T00:00:00Z")
	before, _ := os.ReadFile(filepath.Join(dir, "new.json"))
	now := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)

	var out bytes.Buffer
	if err := runRunsResolve(&out, dir, []string{"old"}, now); err != nil {
		t.Fatal(err)
	}
	if got := runs.Solved(dir); len(got) != 1 || !got["new"] {
		t.Fatalf("solved = %v; want the Ticket's latest Run", got)
	}
	if !strings.Contains(out.String(), "github:o/r#1 is marked solved") {
		t.Fatalf("output = %q", out.String())
	}
	if after, _ := os.ReadFile(filepath.Join(dir, "new.json")); string(after) != string(before) {
		t.Fatal("resolve changed the Run record")
	}

	out.Reset()
	if err := runRunsResolve(&out, dir, []string{"github:o/r#1", "--undo", "--json"}, now); err != nil {
		t.Fatal(err)
	}
	var result resolveResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("json: %v in %q", err, out.String())
	}
	if result != (resolveResult{Ticket: "github:o/r#1", Run: "new", Solved: false, Changed: true}) {
		t.Fatalf("result = %+v", result)
	}
	if got := runs.Solved(dir); len(got) != 0 {
		t.Fatalf("solved after --undo = %v", got)
	}
}

func TestRunsResolveRefusesAnIdItCannotResolve(t *testing.T) {
	dir := t.TempDir()
	writeRunRecord(t, dir, "a", "github:o/r#1", "2026-01-01T00:00:00Z")
	err := runRunsResolve(&bytes.Buffer{}, dir, []string{"nope"}, time.Now())
	if err == nil || !strings.Contains(err.Error(), `"nope"`) || ExitCode(err) == 0 {
		t.Fatalf("err = %v", err)
	}
	if err := runRunsResolve(&bytes.Buffer{}, dir, nil, time.Now()); err == nil {
		t.Fatal("resolve with no id was accepted")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("a refused resolve wrote %d files", len(entries)-1)
	}
}
