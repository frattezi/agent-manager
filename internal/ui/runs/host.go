package runs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// HostVersion is the only host.json version this reader understands.
const HostVersion = 1

// HostState is the subset of Fractory's host.json the manager shows. Fields it
// does not name are ignored.
type HostState struct {
	Version   int       `json:"version"`
	Held      bool      `json:"held"`
	Reason    string    `json:"reason"`
	HeldSince time.Time `json:"heldSince"`
	Queued    int       `json:"queued"`
	UpdatedAt time.Time `json:"updatedAt"`
	// HeldTickets are the Tickets Fractory holds for their blockers; absent
	// from host state written before it listed them.
	HeldTickets []HeldTicket `json:"heldTickets"`
}

// HeldTicket is a Ticket Fractory holds, with the blockers it lists for it.
type HeldTicket struct {
	Repo      string    `json:"repo"`
	TicketRef string    `json:"ticketRef"`
	BlockedBy []Blocker `json:"blockedBy"`
}

// Blocker is one blocker of a held Ticket and why it holds it, in Fractory's
// words: "not done: its latest Run failed".
type Blocker struct {
	Ref    string `json:"ref"`
	Reason string `json:"reason"`
}

// HostPath is `host.json` beside the registry: $FRACTORY_HOME/host.json.
func HostPath(dir string) string { return filepath.Join(filepath.Dir(dir), "host.json") }

// ParseHost decodes host.json, refusing another version.
func ParseHost(data []byte) (HostState, error) {
	var state HostState
	if err := json.Unmarshal(data, &state); err != nil {
		return HostState{}, err
	}
	if state.Version != HostVersion {
		return HostState{}, errors.New("unsupported host state version")
	}
	return state, nil
}

// LoadHost reads the host state beside the registry dir. A missing or invalid
// file is nil: no Held line.
func LoadHost(dir string) *HostState {
	data, err := os.ReadFile(HostPath(dir))
	if err != nil {
		return nil
	}
	state, err := ParseHost(data)
	if err != nil {
		return nil
	}
	return &state
}

// HeldLine is the line shown above the Runs while the queue is Held, empty
// when it is not: "Held 42 min: Docker memory 0.8 GB free < 1 GB".
func HeldLine(state *HostState, now time.Time) string {
	if state == nil || !state.Held {
		return ""
	}
	line := "Held"
	if !state.HeldSince.IsZero() {
		line += " " + heldFor(now.Sub(state.HeldSince))
	}
	if state.Reason != "" {
		line += ": " + state.Reason
	}
	return line
}

func heldFor(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "under 1 min"
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h %d min", int(d/time.Hour), int(d%time.Hour/time.Minute))
	}
	return fmt.Sprintf("%d d %d h", int(d/(24*time.Hour)), int(d%(24*time.Hour)/time.Hour))
}
