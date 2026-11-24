package runs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func readComments(t *testing.T, dir, id string) commentsFile {
	t.Helper()
	data, err := os.ReadFile(CommentsPath(dir, id))
	if err != nil {
		t.Fatal(err)
	}
	var file commentsFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestAddCommentCreatesThenAppends(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := AddComment(dir, "r1", Comment{ID: "c1", Path: "a.go", Line: 3, Body: "rename"}, now); err != nil {
		t.Fatal(err)
	}
	if err := AddComment(dir, "r1", Comment{ID: "c2", Path: "b.go", Line: 9, Body: "why"}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	file := readComments(t, dir, "r1")
	if file.Version != 1 || len(file.Comments) != 2 {
		t.Fatalf("file = %+v", file)
	}
	if file.Comments[0] != (Comment{ID: "c1", Path: "a.go", Line: 3, Body: "rename", CreatedAt: "2026-03-04T05:06:07Z"}) {
		t.Fatalf("first = %+v", file.Comments[0])
	}
	if file.Comments[1].CreatedAt != "2026-03-04T05:07:07Z" {
		t.Fatalf("second = %+v", file.Comments[1])
	}
}

func TestAddCommentLeavesNoStagingFileAndIsNotARun(t *testing.T) {
	dir := t.TempDir()
	if err := AddComment(dir, "r1", Comment{ID: "c1", Path: "a.go", Line: 1, Body: "x"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "r1.comments.json" {
		t.Fatalf("dir holds %v; want only r1.comments.json", entries)
	}
	if runs, _ := Load(dir); len(runs) != 0 {
		t.Fatalf("comments file listed as a run: %+v", runs)
	}
}

func TestAddCommentKeepsAFileItCannotRead(t *testing.T) {
	dir := t.TempDir()
	path := CommentsPath(dir, "r1")
	if err := os.WriteFile(path, []byte(`{broken`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AddComment(dir, "r1", Comment{ID: "c1"}, time.Now()); err == nil {
		t.Fatal("want an error for a corrupt comments file")
	}
	if data, _ := os.ReadFile(path); string(data) != `{broken` {
		t.Fatalf("corrupt file was overwritten: %q", data)
	}
}

func TestAddCommentRefusesAnIDThatEscapesTheDir(t *testing.T) {
	dir := t.TempDir()
	if err := AddComment(dir, "../x", Comment{ID: "c1"}, time.Now()); err == nil {
		t.Fatal("want an error")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "x.comments.json")); err == nil {
		t.Fatal("wrote outside the registry")
	}
}
