package runs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadHostReadsBesideTheRegistry(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "runs")
	body := `{"version":1,"held":true,"reason":"Docker memory 0.8 GB free < 1 GB","heldSince":"2026-11-25T12:00:00Z","readings":{"dockerReachable":true},"queued":2,"updatedAt":"2026-11-25T12:42:00Z","future":1}`
	if err := os.WriteFile(filepath.Join(home, "host.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	state := LoadHost(dir)
	if state == nil || !state.Held || state.Queued != 2 {
		t.Fatalf("state = %+v", state)
	}
	now := time.Date(2026, 11, 25, 12, 42, 30, 0, time.UTC)
	if got, want := HeldLine(state, now), "Held 42 min: Docker memory 0.8 GB free < 1 GB"; got != want {
		t.Fatalf("line = %q, want %q", got, want)
	}
}

func TestLoadHostIgnoresMissingAndInvalidFiles(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "runs")
	if LoadHost(dir) != nil {
		t.Fatal("missing file produced a state")
	}
	for _, body := range []string{`{nope`, `{"version":2,"held":true}`} {
		if err := os.WriteFile(filepath.Join(home, "host.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if LoadHost(dir) != nil {
			t.Fatalf("%q produced a state", body)
		}
	}
}

func TestHeldLine(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		state *HostState
		want  string
	}{
		{"nil", nil, ""},
		{"not held", &HostState{Version: 1}, ""},
		{"no reason", &HostState{Held: true, HeldSince: now.Add(-3 * time.Hour / 2)}, "Held 1 h 30 min"},
		{"fresh", &HostState{Held: true, Reason: "r", HeldSince: now.Add(-10 * time.Second)}, "Held under 1 min: r"},
		{"no since", &HostState{Held: true, Reason: "r"}, "Held: r"},
		{"days", &HostState{Held: true, Reason: "r", HeldSince: now.Add(-50 * time.Hour)}, "Held 2 d 2 h: r"},
	}
	for _, c := range cases {
		if got := HeldLine(c.state, now); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestParseHostReadsHeldTicketsAndToleratesTheirAbsence(t *testing.T) {
	body := `{"version":1,"heldTickets":[{"repo":"/w/api","ticketRef":"o/api#2","blockedBy":[{"ref":"o/api#1","reason":"not done: its latest Run failed"}]}]}`
	state, err := ParseHost([]byte(body))
	if err != nil || len(state.HeldTickets) != 1 {
		t.Fatalf("state = %+v, err = %v", state, err)
	}
	held := state.HeldTickets[0]
	if held.TicketRef != "o/api#2" || held.Repo != "/w/api" || len(held.BlockedBy) != 1 || held.BlockedBy[0].Reason != "not done: its latest Run failed" {
		t.Fatalf("held = %+v", held)
	}
	old, err := ParseHost([]byte(`{"version":1,"held":true}`))
	if err != nil || old.HeldTickets != nil {
		t.Fatalf("old = %+v, err = %v", old, err)
	}
}
