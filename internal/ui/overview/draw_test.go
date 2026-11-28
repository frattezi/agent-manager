package overview

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	"github.com/YoanWai/agent-manager/internal/ui/runs"
)

var testPalette = Palette{
	Swatches: []string{"#ff0000", "#00ff00", "#0000ff"},
	Working:  "#00ffff", Waiting: "#ffff00", Finished: "#00ff00", Errored: "#ff0000", Idle: "#888888",
	Bright: "#ffffff", Text: "#cccccc", Dim: "#999999",
}

func sample() Runs {
	list := []runs.Record{
		run("1", "o/api#1", "api", runs.StatusDone, time.Hour, func(r *runs.Record) {
			r.FinishedAt = now.Add(-time.Hour)
			r.Title = "Add pagination to the list endpoint"
			r.Result = &runs.Result{Summary: "Cursor pagination with a default page of fifty rows."}
			r.PRURL, r.PRState = "https://github.com/o/api/pull/9", "MERGED"
			r.Mode, r.Base = "work", "main"
		}),
		run("2", "o/api#2", "api", runs.StatusFailed, 5*24*time.Hour, finishedAgo(5*24*time.Hour)),
		run("3", "o/api#3", "api", runs.StatusRunning, time.Minute, nil),
		run("4", "o/web#1", "web", runs.StatusNeedsHuman, 2*time.Hour, func(r *runs.Record) { r.Mode = "personal" }),
	}
	return Compute(list, WindowAll, now)
}

func plain(d Drawing) string {
	return strings.NewReplacer("│", " ").Replace(ansi.Strip(strings.Join(d.Lines, "\n")))
}

func TestDrawReadsAtNarrowAndWideWidths(t *testing.T) {
	for _, width := range []int{34, 100} {
		d := Draw(sample(), width, "W", presentation.GlyphModeAdvanced, testPalette)
		text := plain(d)
		for _, want := range []string{"Window all", "W or click for 24h", "api", "WORK", "base main", "Ticket Gate on", "PR Gate on", "web", "PERSONAL", "auto-merge on", "1 running", "1 needs-human",
			"Add pagination to the list endpoint", "Cursor pagination with a default page of fifty rows.", "Tickets", "merged", "Run time"} {
			if !strings.Contains(strings.Join(strings.Fields(text), " "), want) {
				t.Errorf("width %d: Overview lacks %q:\n%s", width, want, text)
			}
		}
		for _, line := range d.Lines {
			if w := ansi.StringWidth(line); w > width {
				t.Errorf("width %d: line is %d cells: %q", width, w, ansi.Strip(line))
			}
		}
		if d.WindowRow != 0 {
			t.Errorf("WindowRow = %d", d.WindowRow)
		}
	}
}

func TestDrawSummaryFollowsItsTitle(t *testing.T) {
	text := plain(Draw(sample(), 80, "W", presentation.GlyphModeAdvanced, testPalette))
	title := strings.Index(text, "Add pagination")
	summary := strings.Index(text, "Cursor pagination")
	if title < 0 || summary < title || strings.Count(text[title:summary], "\n") != 1 {
		t.Errorf("summary is not on the line under its Title:\n%s", text)
	}
}

func TestDrawCompatibleHasNoBraille(t *testing.T) {
	braille := func(s string) bool {
		for _, r := range s {
			if r >= 0x2800 && r <= 0x28FF {
				return true
			}
		}
		return false
	}
	o := sample()
	for range 3 {
		o.Projects[0].Durations = append(o.Projects[0].Durations, 40*time.Minute, 5*time.Minute)
	}
	if !braille(plain(Draw(o, 80, "W", presentation.GlyphModeAdvanced, testPalette))) {
		t.Fatal("Advanced draws no braille, so the Compatible check proves nothing")
	}
	if text := plain(Draw(o, 80, "W", presentation.GlyphModeBasic, testPalette)); braille(text) {
		t.Errorf("Compatible mode drew braille:\n%s", text)
	}
}

func TestDrawEmptyStates(t *testing.T) {
	if text := plain(Draw(Runs{Window: WindowAll}, 60, "W", presentation.GlyphModeAdvanced, testPalette)); !strings.Contains(text, "No Runs yet") {
		t.Errorf("empty all window: %q", text)
	}
	if text := plain(Draw(Runs{Window: Window7d}, 60, "", presentation.GlyphModeAdvanced, testPalette)); !strings.Contains(text, "No Runs in the last 7d") || !strings.Contains(text, "click for 30d") {
		t.Errorf("empty 7d window: %q", text)
	}
}

func TestMergeFoldsLongWindowsIntoWeeks(t *testing.T) {
	bars := make([]Bucket, 100)
	for i := range bars {
		bars[i].Done = 1
	}
	got, factor := merge(bars, false, 30)
	if factor != 7 || len(got) != 15 || got[0].Done != 7 || got[14].Done != 2 {
		t.Errorf("factor %d, %d buckets, first %+v", factor, len(got), got[0])
	}
	if got, factor := merge(bars[:24], true, 10); factor != 3 || len(got) != 8 {
		t.Errorf("hours: factor %d, %d buckets", factor, len(got))
	}
}

func TestDrawCapsALongSummaryToItsFirstParagraph(t *testing.T) {
	long := strings.Repeat("Every gate passed and the change merged cleanly. ", 12) + "\n\nSecond paragraph that must not show."
	o := Compute([]runs.Record{run("1", "o/api#1", "api", runs.StatusDone, time.Hour, func(r *runs.Record) {
		r.FinishedAt = now.Add(-time.Hour)
		r.Title = "Long summary"
		r.Result = &runs.Result{Summary: long}
	})}, WindowAll, now)
	for _, width := range []int{34, 80} {
		text := plain(Draw(o, width, "W", presentation.GlyphModeAdvanced, testPalette))
		if strings.Contains(text, "Second paragraph") {
			t.Errorf("width %d: a later paragraph is drawn:\n%s", width, text)
		}
		after := text[strings.Index(text, "Long summary"):]
		lines := strings.Split(after, "\n")
		n := 0
		for _, line := range lines[1:] {
			if !strings.Contains(line, "Every gate") && !strings.Contains(line, "merged cleanly") && !strings.Contains(line, "…") {
				break
			}
			n++
		}
		if n != excerptLines || !strings.HasSuffix(strings.TrimRight(lines[n], " "), "…") {
			t.Errorf("width %d: summary drew %d lines, want %d ending in …:\n%s", width, n, excerptLines, after)
		}
	}
}

func TestDrawKeepsAShortSummaryWhole(t *testing.T) {
	text := plain(Draw(sample(), 80, "W", presentation.GlyphModeAdvanced, testPalette))
	if strings.Contains(text, "rows. …") || !strings.Contains(text, "Cursor pagination with a default page of fifty rows.") {
		t.Errorf("a one-line summary was cut:\n%s", text)
	}
}
