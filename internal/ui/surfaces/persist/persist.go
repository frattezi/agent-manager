// Package persist is the store as the Agent Surfaces poll reads it.
package persist

import (
	"time"

	"github.com/YoanWai/agent-manager/internal/store"
	"github.com/YoanWai/agent-manager/internal/ui/surfaces"
)

// Store answers surfaces.Store from the manager's store.
type Store struct{ Store *store.Store }

var _ surfaces.Store = Store{}

func (s Store) LatestSurfaces() ([]surfaces.Stored, error) {
	latest, err := s.Store.LatestAgentSurfaces()
	if err != nil {
		return nil, err
	}
	stored := make([]surfaces.Stored, 0, len(latest))
	for _, a := range latest {
		stored = append(stored, surfaces.Stored{ID: a.ID, SessionID: a.SessionID, PublishedAt: a.PublishedAt, Document: a.Document, AnsweredAt: a.AnsweredAt})
	}
	return stored, nil
}

// AnswerSurface records the answer and queues it on the session's inbox,
// the path a message between agents takes, so it is typed in only once the
// pane is at rest and never twice.
func (s Store) AnswerSurface(r surfaces.AnswerRequest) error {
	_, err := s.Store.AnswerAgentSurface(r.SurfaceID, r.Answer, store.InboxMessage{
		SessionID:   r.SessionID,
		SenderName:  "the user",
		Body:        r.Answer,
		Fingerprint: "surface:" + r.SurfaceID,
		SentAt:      time.Now(),
		SurfaceName: r.SurfaceName,
	}, store.DefaultInboxLimits)
	return err
}

func Services(st *store.Store) surfaces.Services {
	return surfaces.Services{Store: Store{Store: st}}
}
