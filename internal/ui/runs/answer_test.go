package runs

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestWriteAnswerWritesTheContract(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.FixedZone("x", -3*3600))
	if err := WriteAnswer(dir, "run1", "use the orders table\nnot users", now); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "run1.answer.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"version":1,"answeredAt":"2026-11-25T15:00:00Z","answer":"use the orders table\nnot users"}` + "\n"
	if string(data) != want {
		t.Fatalf("file = %q, want %q", data, want)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("dir holds %d files, want only the answer (no temp files)", len(entries))
	}
}

func TestWriteAnswerOverwritesTheWaitingAnswer(t *testing.T) {
	dir := t.TempDir()
	if err := WriteAnswer(dir, "run1", "first", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := WriteAnswer(dir, "run1", "second", time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := Answers(dir); len(got) != 1 || got["run1"] != "second" {
		t.Fatalf("Answers = %v, want the second answer", got)
	}
}

func TestWriteAnswerRefusesAnEmptyAnswerAndABadID(t *testing.T) {
	dir := t.TempDir()
	for _, answer := range []string{"", "  \n\t "} {
		if err := WriteAnswer(dir, "run1", answer, time.Now()); !errors.Is(err, ErrEmptyAnswer) {
			t.Fatalf("WriteAnswer(%q) = %v, want ErrEmptyAnswer", answer, err)
		}
	}
	for _, id := range []string{"", "a/b"} {
		if err := WriteAnswer(dir, id, "yes", time.Now()); err == nil {
			t.Fatalf("id %q was accepted", id)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("a refused answer left %d files", len(entries))
	}
}

func TestWriteAnswerToAnUnwritableDirLeavesNoFile(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions do not bind here")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if err := WriteAnswer(dir, "run1", "yes", time.Now()); err == nil {
		t.Fatal("a write into a read-only dir succeeded")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("the failed write left %v", entries)
	}
}

func TestAnswersSkipsFilesFractoryWouldIgnoreAndLoadIgnoresAnswers(t *testing.T) {
	dir := t.TempDir()
	writeRun(t, dir, "a.json", record("a", "needs-human", "2026-01-02T00:00:00Z", ""))
	if err := WriteAnswer(dir, "a", "yes", time.Now()); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"b.answer.json": `{"version":2,"answeredAt":"2026-01-02T00:00:00Z","answer":"x"}`,
		"c.answer.json": `{"version":1,"answeredAt":"2026-01-02T00:00:00Z","answer":""}`,
		"d.answer.json": `{"version":1,`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := Answers(dir); len(got) != 1 || got["a"] != "yes" {
		t.Fatalf("Answers = %v, want only a", got)
	}
	if got, _ := Load(dir); len(got) != 1 {
		t.Fatalf("Load = %d records, want 1", len(got))
	}
	if got := Answers(filepath.Join(dir, "absent")); len(got) != 0 {
		t.Fatalf("Answers of a missing dir = %v", got)
	}
}

func TestOnlyANeedsHumanRunCanBeAnswered(t *testing.T) {
	for _, status := range []string{StatusQueued, StatusRunning, StatusDone, StatusFailed, StatusNeedsHuman} {
		if got, want := (Record{Status: status}).CanAnswer(), status == StatusNeedsHuman; got != want {
			t.Fatalf("CanAnswer(%s) = %v", status, got)
		}
	}
}
