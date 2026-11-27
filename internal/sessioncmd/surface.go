package sessioncmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/YoanWai/agent-manager/internal/store"
	"github.com/YoanWai/agent-manager/internal/ui/a2ui"
	"github.com/google/uuid"
)

// RetainedSurfaces is how far back a reader looks through one session's
// rounds: a Goal is grilled in a handful of them, and a session rarely
// carries more than a few Goals before it is deleted. Past that a round is
// history nobody opens, so the oldest goes when a new one arrives.
const RetainedSurfaces = 50

// DefaultSurfaceWidth is the width a surface is drawn at when the reader
// names none: a terminal's classic width, which every terminal reaches.
const DefaultSurfaceWidth = 80

// AgentSurface is one Agent Surface a session published, as either front
// lists it.
type AgentSurface struct {
	ID          string     `json:"id" jsonschema:"stored surface id, the one surface show takes"`
	SessionID   string     `json:"session_id" jsonschema:"session that published it"`
	SessionName string     `json:"session_name,omitempty" jsonschema:"name of the session that published it"`
	SurfaceID   string     `json:"surface_id" jsonschema:"the surfaceId the document names"`
	PublishedAt time.Time  `json:"published_at"`
	Answered    bool       `json:"answered" jsonschema:"whether the owner has answered it"`
	AnsweredAt  *time.Time `json:"answered_at,omitempty"`
	Answer      string     `json:"answer,omitempty" jsonschema:"what the owner answered, once they have"`
}

// RenderedSurface is a stored surface drawn as text at a width.
type RenderedSurface struct {
	Surface AgentSurface `json:"surface"`
	Width   int          `json:"width"`
	Text    string       `json:"text" jsonschema:"the surface drawn as plain text"`
}

// PublishSurface validates an A2UI document and stores it against the
// calling session. Only an agent session publishes: the answer travels back
// into a conversation, which a terminal does not have.
func (s *Sessions) PublishSurface(sessionID string, document []byte) (AgentSurface, error) {
	runtime, err := s.open()
	if err != nil {
		return AgentSurface{}, err
	}
	defer runtime.Close()
	caller, err := runtime.caller(sessionID)
	if err != nil {
		return AgentSurface{}, fmt.Errorf("an Agent Surface is published by the session that asks, so it needs one: %w", err)
	}
	if runtime.cfg.Tools[caller.Tool].Shell {
		return AgentSurface{}, fmt.Errorf("%s is a terminal, which can be sent to but never asked: an answer to its surface would have no conversation to return to, so only an agent session publishes one", caller.Name)
	}
	doc, err := a2ui.Parse(document)
	if err != nil {
		return AgentSurface{}, fmt.Errorf("surface refused and nothing was stored; fix the document and publish again: %w", err)
	}
	stored := store.AgentSurface{
		ID:          uuid.NewString()[:8],
		SessionID:   caller.ID,
		PublishedAt: time.Now(),
		Document:    string(document),
	}
	if err := runtime.store.PublishAgentSurface(stored, RetainedSurfaces); err != nil {
		return AgentSurface{}, err
	}
	return surfaceRecord(stored, caller.Name, doc.SurfaceID), nil
}

// Surfaces lists a session's stored surfaces oldest first: the caller's own
// when targetID is empty, so an agent reads its rounds back, or the named
// session's, so the owner can read them from any shell.
func (s *Sessions) Surfaces(sessionID, targetID string) ([]AgentSurface, error) {
	runtime, err := s.open()
	if err != nil {
		return nil, err
	}
	defer runtime.Close()
	owner, err := runtime.surfaceOwner(sessionID, targetID)
	if err != nil {
		return nil, err
	}
	stored, err := runtime.store.AgentSurfaces(owner.ID)
	if err != nil {
		return nil, err
	}
	listed := make([]AgentSurface, 0, len(stored))
	for _, surface := range stored {
		listed = append(listed, surfaceRecord(surface, owner.Name, storedSurfaceID(surface)))
	}
	return listed, nil
}

// ShowSurface draws one stored surface as text at width, or at
// DefaultSurfaceWidth when width is zero.
func (s *Sessions) ShowSurface(sessionID, id string, width int) (RenderedSurface, error) {
	if width < 0 {
		return RenderedSurface{}, fmt.Errorf("width %d is negative; omit it to draw at %d", width, DefaultSurfaceWidth)
	}
	if width == 0 {
		width = DefaultSurfaceWidth
	}
	runtime, err := s.open()
	if err != nil {
		return RenderedSurface{}, err
	}
	defer runtime.Close()
	if _, err := runtime.optionalCaller(sessionID); err != nil {
		return RenderedSurface{}, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return RenderedSurface{}, fmt.Errorf("surface id is empty; call %s to get one", runtime.words.ListSurfaces)
	}
	stored, err := runtime.store.AgentSurface(id)
	if errors.Is(err, store.ErrAgentSurfaceNotFound) {
		return RenderedSurface{}, fmt.Errorf("surface %s does not exist; call %s for current ids", id, runtime.words.ListSurfaces)
	}
	if err != nil {
		return RenderedSurface{}, err
	}
	names, err := runtime.sessionNames()
	if err != nil {
		return RenderedSurface{}, err
	}
	// The document was validated when it was stored; a build that reads
	// another version since is told so rather than drawing a guess.
	doc, err := a2ui.Parse([]byte(stored.Document))
	if err != nil {
		return RenderedSurface{}, fmt.Errorf("surface %s cannot be drawn by this build: %w", id, err)
	}
	return RenderedSurface{
		Surface: surfaceRecord(stored, names[stored.SessionID], doc.SurfaceID),
		Width:   width,
		Text:    a2ui.Render(doc, width),
	}, nil
}

// surfaceOwner is the session whose surfaces a read lists. An outside
// shell names one; a session that names none reads its own.
func (r *runtime) surfaceOwner(sessionID, targetID string) (store.Session, error) {
	if strings.TrimSpace(targetID) == "" {
		caller, err := r.caller(sessionID)
		if err != nil {
			return store.Session{}, fmt.Errorf("name the session whose surfaces to list, or run this from inside one: %w", err)
		}
		return caller, nil
	}
	if _, err := r.optionalCaller(sessionID); err != nil {
		return store.Session{}, err
	}
	return r.agent(targetID)
}

func storedSurfaceID(surface store.AgentSurface) string {
	doc, err := a2ui.Parse([]byte(surface.Document))
	if err != nil {
		return ""
	}
	return doc.SurfaceID
}

func surfaceRecord(surface store.AgentSurface, sessionName, surfaceID string) AgentSurface {
	record := AgentSurface{
		ID:          surface.ID,
		SessionID:   surface.SessionID,
		SessionName: sessionName,
		SurfaceID:   surfaceID,
		PublishedAt: surface.PublishedAt.UTC(),
		Answered:    !surface.AnsweredAt.IsZero(),
		Answer:      surface.Answer,
	}
	if record.Answered {
		answered := surface.AnsweredAt.UTC()
		record.AnsweredAt = &answered
	}
	return record
}

// FormatSurface is the one line both fronts print for a stored surface.
func FormatSurface(surface AgentSurface) string {
	state := "unanswered"
	if surface.Answered {
		state = "answered"
	}
	name := surface.SurfaceID
	if name == "" {
		name = "unreadable"
	}
	return fmt.Sprintf("%s (%s) [%s] published %s", name, surface.ID, state, surface.PublishedAt.Format(time.RFC3339))
}

func FormatSurfacePublished(surface AgentSurface) string {
	return "published " + FormatSurface(surface) + " for " + surface.SessionName
}

func FormatSurfaceList(surfaces []AgentSurface) string {
	if len(surfaces) == 0 {
		return "no surfaces published"
	}
	lines := make([]string, 0, len(surfaces))
	for _, surface := range surfaces {
		lines = append(lines, FormatSurface(surface))
	}
	return strings.Join(lines, "\n")
}
