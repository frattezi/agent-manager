// Package persist is the store as the Agent Surfaces poll reads it.
package persist

import (
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

func Services(st *store.Store) surfaces.Services {
	return surfaces.Services{Store: Store{Store: st}}
}
