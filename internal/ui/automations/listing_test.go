package automations

import (
	"testing"
	"time"
)

func listingDoc(id, cron string) []byte {
	return []byte(`{"version":1,"id":"` + id + `","name":"` + id + ` chore","kind":"script","repoPath":"/src","cron":"` + cron + `","timezone":"UTC","mode":"work","tool":"","flags":[],"entry":"run.sh","retention":3}`)
}

func TestEntriesPairEachAutomationWithItsRecentFiringsNewestFirst(t *testing.T) {
	at := time.Date(2026, 11, 26, 7, 0, 0, 0, time.UTC)
	rows := []Row{
		{ID: "a", Name: "stored a", Document: listingDoc("a", "0 7 * * *"), Enabled: true},
		{ID: "b", Name: "stored b", Document: []byte("{"), Enabled: true},
	}
	var records []Record
	for i := range RecentFirings + 2 {
		records = append(records, Record{ID: string(rune('a' + i)), AutomationID: "a", FiredFor: at.Add(-time.Duration(i) * 24 * time.Hour), Status: "done"})
	}
	records = append(records, Record{ID: "other", AutomationID: "gone", FiredFor: at, Status: "done"})
	entries := Entries(rows, records)
	if len(entries) != 2 || entries[0].Name != "a chore" || entries[1].Name != "stored b" || entries[1].Err == nil {
		t.Fatalf("entries = %+v", entries)
	}
	if got := len(entries[0].Firings); got != RecentFirings {
		t.Fatalf("listed %d firings, want %d", got, RecentFirings)
	}
	if last, ok := entries[0].Last(); !ok || !last.FiredFor.Equal(at) {
		t.Fatalf("last = %+v, %v", last, ok)
	}
	if _, ok := entries[1].Last(); ok {
		t.Fatal("an Automation with no records has a last firing")
	}
}

func TestNextIsTheComingInstantOnlyWhileEnabled(t *testing.T) {
	now := time.Date(2026, 11, 26, 8, 0, 0, 0, time.UTC)
	entry := Entries([]Row{{ID: "a", Document: listingDoc("a", "0 7 * * *"), Enabled: true}}, nil)[0]
	if next, ok := entry.Next(now); !ok || !next.Equal(time.Date(2026, 11, 27, 7, 0, 0, 0, time.UTC)) {
		t.Fatalf("next = %v, %v", next, ok)
	}
	entry.Enabled = false
	if _, ok := entry.Next(now); ok {
		t.Fatal("a disabled Automation has a next fire time")
	}
}

func TestFiringOutcomeAndDuration(t *testing.T) {
	start := time.Date(2026, 11, 26, 7, 0, 0, 0, time.UTC)
	done := Record{Status: "done", StartedAt: start, UpdatedAt: start.Add(90 * time.Second), Result: &Result{Outcome: "failed"}}
	if done.Outcome() != "failed" {
		t.Fatalf("outcome = %q", done.Outcome())
	}
	if took, ok := done.Duration(start.Add(time.Hour)); !ok || took != 90*time.Second {
		t.Fatalf("duration = %v, %v", took, ok)
	}
	running := Record{Status: StatusRunning, StartedAt: start, UpdatedAt: start.Add(time.Second)}
	if took, ok := running.Duration(start.Add(time.Minute)); !ok || took != time.Minute || running.Finished() {
		t.Fatalf("running duration = %v, %v", took, ok)
	}
	if _, ok := (Record{Status: StatusQueued}).Duration(start); ok {
		t.Fatal("a queued firing has a duration")
	}
}
