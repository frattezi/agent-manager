package sessioncmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/YoanWai/agent-manager/internal/store"
	"github.com/YoanWai/agent-manager/internal/ui/a2ui"
	"github.com/YoanWai/agent-manager/internal/ui/presentation"
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
	// Document is the A2UI document exactly as it was published, for a host
	// that lays the surface out itself. It is carried as a string so the
	// bytes the session wrote come back unchanged, not re-encoded.
	Document string `json:"document,omitempty" jsonschema:"the A2UI document as it was published, byte for byte, when it was asked for"`
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
// DefaultSurfaceWidth when width is zero, and carries the published document
// beside the text when withDocument is set.
func (s *Sessions) ShowSurface(sessionID, id string, width int, withDocument bool) (RenderedSurface, error) {
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
	stored, err := runtime.storedSurface(id)
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
	shown := RenderedSurface{
		Surface: surfaceRecord(stored, names[stored.SessionID], doc.SurfaceID),
		Width:   width,
		Text:    a2ui.Render(doc, width, presentation.ChromeFor(presentation.DefaultGlyphMode)),
	}
	if withDocument {
		shown.Document = stored.Document
	}
	return shown, nil
}

// AnswerSurface records an answer to a surface from outside the manager's
// card, for a host that drew the round itself. It has the effect sending
// from the card has: the round is marked answered and keeps its answer, and
// the answer is queued on the asking session's inbox in the same message,
// to be typed in once its pane is at rest. With delivered, the asking session
// reports an answer it already received as a user turn: the round is recorded
// the same way but nothing is queued, since the conversation has it.
func (s *Sessions) AnswerSurface(sessionID, id, answer string, delivered bool) (AgentSurface, error) {
	runtime, err := s.open()
	if err != nil {
		return AgentSurface{}, err
	}
	defer runtime.Close()
	caller, err := runtime.optionalCaller(sessionID)
	if err != nil {
		return AgentSurface{}, err
	}
	if strings.TrimSpace(answer) == "" {
		return AgentSurface{}, errors.New("the answer is empty; a round is answered with what the user chose, so pass it as text")
	}
	stored, err := runtime.storedSurface(id)
	if err != nil {
		return AgentSurface{}, err
	}
	if delivered && caller.ID != stored.SessionID {
		return AgentSurface{}, fmt.Errorf("surface %s was asked by another session, so an answer cannot be marked delivered from here: it would never reach that conversation; answer without the delivered option to queue it", stored.ID)
	}
	if !delivered && caller.ID == stored.SessionID {
		return AgentSurface{}, fmt.Errorf("surface %s is your own question to the user, so only the user answers it", stored.ID)
	}
	surfaceName := storedSurfaceID(stored)
	at := time.Now()
	msg := store.SurfaceAnswerMessage(stored.SessionID, stored.ID, surfaceName, answer, at)
	if delivered {
		err = runtime.store.RecordAgentSurfaceAnswer(stored.ID, answer, msg)
	} else {
		_, err = runtime.store.AnswerAgentSurface(stored.ID, answer, msg, store.DefaultInboxLimits)
	}
	switch {
	case errors.Is(err, store.ErrAgentSurfaceAnswered):
		return AgentSurface{}, fmt.Errorf("surface %s is already answered, and an answer reaches its session once, so this one was not recorded", stored.ID)
	case errors.Is(err, store.ErrAgentSurfaceNotFound):
		return AgentSurface{}, fmt.Errorf("surface %s does not exist; call %s for current ids", stored.ID, runtime.words.ListSurfaces)
	case err != nil:
		return AgentSurface{}, fmt.Errorf("surface %s was not answered: %w", stored.ID, err)
	}
	names, err := runtime.sessionNames()
	if err != nil {
		return AgentSurface{}, err
	}
	stored.AnsweredAt, stored.Answer = at, answer
	return surfaceRecord(stored, names[stored.SessionID], surfaceName), nil
}

// storedSurface reads one surface by the id publish or list handed out.
func (r *runtime) storedSurface(id string) (store.AgentSurface, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return store.AgentSurface{}, fmt.Errorf("surface id is empty; call %s to get one", r.words.ListSurfaces)
	}
	stored, err := r.store.AgentSurface(id)
	if errors.Is(err, store.ErrAgentSurfaceNotFound) {
		return store.AgentSurface{}, fmt.Errorf("surface %s does not exist; call %s for current ids", id, r.words.ListSurfaces)
	}
	return stored, err
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

// FormatSurfaceShown is what both fronts print for surface show: the drawn
// text, then the published document as written when it was asked for.
func FormatSurfaceShown(shown RenderedSurface) string {
	text := strings.TrimRight(shown.Text, "\n")
	if shown.Document == "" {
		return text
	}
	return text + "\n\n" + shown.Document
}

func FormatSurfaceAnswered(surface AgentSurface) string {
	return "answered " + FormatSurface(surface) + "; the answer is queued for " + surface.SessionName + " and typed in once it is at rest, unless it was recorded as delivered"
}

func FormatSurfaceList(surfaces []AgentSurface) string {
	if len(surfaces) == 0 {
		return "no surfaces published"
	}
	lines := make([]string, 0, len(surfaces))
	for _, surface := range surfaces {
		lines = append(lines, FormatSurface(surface))
		if surface.Answered {
			for _, line := range strings.Split(surface.Answer, "\n") {
				lines = append(lines, "  "+line)
			}
		}
	}
	return strings.Join(lines, "\n")
}
