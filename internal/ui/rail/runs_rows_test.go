package rail_test

import (
	"strings"
	"testing"

	"github.com/YoanWai/agent-manager/internal/ui/rail"
	"github.com/charmbracelet/x/ansi"
)

func runRowAt(t *testing.T, run rail.Run, width int) string {
	t.Helper()
	model := rail.New(nil)
	model.Reconcile(rail.Snapshot{Runs: []rail.Run{run}})
	for _, line := range model.Render(renderContext(width, 8), rail.Frame{}).Lines {
		if text := ansi.Strip(line.Text); strings.Contains(text, "done") {
			return strings.TrimRight(text, " ")
		}
	}
	t.Fatalf("no run row at width %d", width)
	return ""
}

func TestRunRowReadsAsWork(t *testing.T) {
	run := rail.Run{ID: "r", Ticket: "github:o/r#270", Status: "done", Mode: "work", Project: "esquerda", Title: "Wire the favicon into the footer", Number: "PR 293"}
	wide := runRowAt(t, run, 80)
	for _, want := range []string{"esquerda", "Wire the favicon into the footer", "PR 293", "W done"} {
		if !strings.Contains(wide, want) {
			t.Errorf("wide row lacks %q: %q", want, wide)
		}
	}
	if strings.Contains(wide, "github:") {
		t.Errorf("wide row shows the forge: %q", wide)
	}
	narrow := runRowAt(t, run, 30)
	if strings.Contains(narrow, "esquerda") {
		t.Errorf("narrow row keeps the Project: %q", narrow)
	}
	if !strings.Contains(narrow, "W done") || !strings.Contains(narrow, "Wire") {
		t.Errorf("narrow row lost badge, status or Title: %q", narrow)
	}
	if got := ansi.StringWidth(narrow); got > 30 {
		t.Errorf("narrow row is %d columns: %q", got, narrow)
	}
}

func TestRunRowDropsNumberBeforeTruncatingTitle(t *testing.T) {
	run := rail.Run{ID: "r", Ticket: "github:o/r#270", Status: "done", Mode: "work", Title: "A very long title that cannot fit", Number: "#270"}
	row := runRowAt(t, run, 30)
	if strings.Contains(row, "#270") || !strings.Contains(row, "…") || !strings.Contains(row, "W done") {
		t.Errorf("row = %q", row)
	}
}

func TestRunRowFallsBackToTicketRef(t *testing.T) {
	if row := runRowAt(t, rail.Run{ID: "r", Ticket: "jira:ABC-1", Status: "done"}, 60); !strings.Contains(row, "jira:ABC-1") {
		t.Errorf("row = %q", row)
	}
}

func TestRunSearchMatchesProjectAndTitle(t *testing.T) {
	model := rail.New(nil)
	model.Reconcile(rail.Snapshot{Runs: []rail.Run{
		{ID: "a", Ticket: "jira:A-1", Status: "done", Project: "esquerda", Title: "Fix footer"},
		{ID: "b", Ticket: "jira:B-1", Status: "done", Project: "other", Title: "Other"},
	}})
	for _, q := range []string{"esquerda", "footer", "a-1"} {
		model.SetSearch(q, false)
		n := 0
		for _, row := range model.Rows() {
			if row.Kind == rail.RunRow {
				n++
			}
		}
		if n != 1 {
			t.Errorf("search %q listed %d Runs", q, n)
		}
	}
}
