// Package surfaces is what the manager keeps of the Agent Surfaces its
// sessions published: which one is current for each session and whether it
// waits on the owner, and the card that draws it over the list. The format
// is internal/ui/a2ui's; the store behind the poll is persist's.
package surfaces

import "time"

// Stored is one published surface as the store keeps it.
type Stored struct {
	ID          string
	SessionID   string
	PublishedAt time.Time
	Document    string
	AnsweredAt  time.Time
}

// Store is where the poll reads each session's newest surface.
type Store interface {
	LatestSurfaces() ([]Stored, error)
}

// Services is what the poll runs against.
type Services struct {
	Store Store
}

// ReadRequest reads which sessions are asking.
type ReadRequest struct{}

func (ReadRequest) EffectName() string { return "Agent Surfaces read" }

func (ReadRequest) Run(services Services) (any, error) {
	latest, err := services.Store.LatestSurfaces()
	if err != nil {
		return Inbox{}, err
	}
	return NewInbox(latest), nil
}

// Inbox is each session's current surface while it waits on the owner. A
// session's current surface is its newest: a later round replaces an
// unanswered one, and an answered one leaves the session asking nothing.
type Inbox struct {
	asking map[string]Stored
}

// NewInbox keeps, of each session's newest surface, the unanswered ones.
func NewInbox(latest []Stored) Inbox {
	inbox := Inbox{asking: map[string]Stored{}}
	for _, s := range latest {
		if current, ok := inbox.asking[s.SessionID]; ok && !s.PublishedAt.After(current.PublishedAt) {
			continue
		}
		inbox.asking[s.SessionID] = s
	}
	for id, s := range inbox.asking {
		if !s.AnsweredAt.IsZero() {
			delete(inbox.asking, id)
		}
	}
	return inbox
}

// Asking is whether the session has a surface waiting on the owner.
func (i Inbox) Asking(sessionID string) bool {
	_, ok := i.asking[sessionID]
	return ok
}

// Current is the surface the session waits on the owner with.
func (i Inbox) Current(sessionID string) (Stored, bool) {
	s, ok := i.asking[sessionID]
	return s, ok
}
