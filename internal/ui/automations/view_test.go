package automations

import (
	"testing"
	"time"
)

func TestOutcomeIsTheResultOnceThereIsOne(t *testing.T) {
	record := Record{Status: StatusRunning}
	if got := record.Outcome(); got != StatusRunning {
		t.Fatalf("outcome = %q", got)
	}
	record.Status, record.Result = "done", &Result{Outcome: "failed"}
	if got := record.Outcome(); got != "failed" {
		t.Fatalf("outcome = %q", got)
	}
}

func TestDurationRunsToNowUntilTheRecordFinishes(t *testing.T) {
	start := time.Date(2026, 11, 26, 7, 0, 0, 0, time.UTC)
	now := start.Add(5 * time.Minute)
	if _, ok := (Record{Status: StatusQueued}).Duration(now); ok {
		t.Fatal("a queued record has a duration")
	}
	running := Record{Status: StatusRunning, StartedAt: start, UpdatedAt: start.Add(time.Minute)}
	if got, _ := running.Duration(now); got != 5*time.Minute {
		t.Fatalf("running = %v", got)
	}
	done := Record{Status: "done", StartedAt: start, UpdatedAt: start.Add(42 * time.Second)}
	if got, _ := done.Duration(now); got != 42*time.Second {
		t.Fatalf("done = %v", got)
	}
}

func TestFormatDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{
		42 * time.Second:                           "42s",
		3*time.Minute + 12*time.Second:             "3m12s",
		time.Hour + 5*time.Minute + 59*time.Second: "1h05m",
	} {
		if got := FormatDuration(d); got != want {
			t.Errorf("FormatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestByAutomationKeepsTheOrderGiven(t *testing.T) {
	grouped := ByAutomation([]Record{{ID: "3", AutomationID: "a"}, {ID: "2", AutomationID: "b"}, {ID: "1", AutomationID: "a"}})
	if a := grouped["a"]; len(a) != 2 || a[0].ID != "3" || a[1].ID != "1" {
		t.Fatalf("a = %+v", a)
	}
	if b := grouped["b"]; len(b) != 1 {
		t.Fatalf("b = %+v", b)
	}
}

func TestNextFireIsTheFirstInstantAfterNow(t *testing.T) {
	now := time.Date(2026, 11, 26, 8, 0, 0, 0, time.UTC)
	next, err := NextFire(Config{Cron: "0 7 * * *", Timezone: "Europe/Lisbon"}, now)
	if err != nil || !next.Equal(time.Date(2026, 11, 27, 7, 0, 0, 0, time.UTC)) {
		t.Fatalf("next = %v, %v", next, err)
	}
	if _, err := NextFire(Config{Cron: "not cron", Timezone: "UTC"}, now); err == nil {
		t.Fatal("an unreadable schedule has a next instant")
	}
}

func TestDescribeReadsADocumentWithoutValidatingIt(t *testing.T) {
	config, err := Describe([]byte(`{"version":2,"name":"x","mode":"work","later":true}`))
	if err != nil || config.Mode != ModeWork || config.Name != "x" {
		t.Fatalf("config = %+v, %v", config, err)
	}
}
