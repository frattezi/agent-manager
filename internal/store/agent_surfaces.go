package store

import (
	"database/sql"
	"errors"
	"time"
)

// ErrAgentSurfaceNotFound reports an id no stored Agent Surface carries.
var ErrAgentSurfaceNotFound = errors.New("agent surface does not exist")

// AgentSurface is one Agent Surface a session published. The document is
// kept as written; internal/ui/a2ui validates it before it is stored.
type AgentSurface struct {
	ID          string
	SessionID   string
	PublishedAt time.Time
	Document    string
	AnsweredAt  time.Time
	Answer      string
}

const agentSurfaceColumns = `id, session_id, published_at, document, answered_at, answer`

func scanAgentSurface(row interface{ Scan(...any) error }) (AgentSurface, error) {
	var a AgentSurface
	var published, answered int64
	if err := row.Scan(&a.ID, &a.SessionID, &published, &a.Document, &answered, &a.Answer); err != nil {
		return AgentSurface{}, err
	}
	a.PublishedAt = decodeTime(published)
	a.AnsweredAt = decodeTime(answered)
	return a, nil
}

// PublishAgentSurface stores a surface and trims its session's history to
// the newest keep, in one transaction, so a reader never sees the session
// over its cap.
func (s *Store) PublishAgentSurface(a AgentSurface, keep int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO agent_surfaces (`+agentSurfaceColumns+`) VALUES (?, ?, ?, ?, ?, ?)`,
		a.ID, a.SessionID, encodeTime(a.PublishedAt), a.Document, encodeTime(a.AnsweredAt), a.Answer); err != nil {
		return err
	}
	if _, err := tx.Exec(`
DELETE FROM agent_surfaces WHERE session_id = ? AND rowid NOT IN (
	SELECT rowid FROM agent_surfaces WHERE session_id = ? ORDER BY published_at DESC, rowid DESC LIMIT ?
)`, a.SessionID, a.SessionID, keep); err != nil {
		return err
	}
	return tx.Commit()
}

// AgentSurfaces lists a session's stored surfaces, oldest first, which is
// the order its rounds read back in.
func (s *Store) AgentSurfaces(sessionID string) ([]AgentSurface, error) {
	rows, err := s.db.Query(`SELECT `+agentSurfaceColumns+` FROM agent_surfaces WHERE session_id = ? ORDER BY published_at, rowid`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	surfaces := []AgentSurface{}
	for rows.Next() {
		a, err := scanAgentSurface(rows)
		if err != nil {
			return nil, err
		}
		surfaces = append(surfaces, a)
	}
	return surfaces, rows.Err()
}

func (s *Store) AgentSurface(id string) (AgentSurface, error) {
	a, err := scanAgentSurface(s.db.QueryRow(`SELECT `+agentSurfaceColumns+` FROM agent_surfaces WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return AgentSurface{}, ErrAgentSurfaceNotFound
	}
	return a, err
}

// LatestAgentSurfaces is each session's newest surface, answered or not,
// keyed by session, in one query for the manager's poll.
func (s *Store) LatestAgentSurfaces() (map[string]AgentSurface, error) {
	rows, err := s.db.Query(`SELECT ` + agentSurfaceColumns + ` FROM agent_surfaces a WHERE NOT EXISTS (
	SELECT 1 FROM agent_surfaces b WHERE b.session_id = a.session_id
	AND (b.published_at > a.published_at OR (b.published_at = a.published_at AND b.rowid > a.rowid))
)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	latest := map[string]AgentSurface{}
	for rows.Next() {
		a, err := scanAgentSurface(rows)
		if err != nil {
			return nil, err
		}
		latest[a.SessionID] = a
	}
	return latest, rows.Err()
}

// ErrAgentSurfaceAnswered refuses a second answer to one surface: the first
// is already on its way into the session.
var ErrAgentSurfaceAnswered = errors.New("agent surface is already answered")

// AnswerAgentSurface records the owner's answer to a surface and queues
// msg, which carries it, on the session's inbox, in one transaction: a
// surface is never marked answered without its answer queued, and an answer
// the inbox refuses leaves the round unanswered.
func (s *Store) AnswerAgentSurface(id, answer string, msg InboxMessage, limits InboxLimits) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE agent_surfaces SET answered_at = ?, answer = ? WHERE id = ? AND session_id = ? AND answered_at = 0`,
		encodeTime(msg.SentAt), answer, id, msg.SessionID)
	if err != nil {
		return 0, err
	}
	if affected, err := res.RowsAffected(); err != nil {
		return 0, err
	} else if affected == 0 {
		var answered int64
		err := tx.QueryRow(`SELECT answered_at FROM agent_surfaces WHERE id = ? AND session_id = ?`, id, msg.SessionID).Scan(&answered)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrAgentSurfaceNotFound
		}
		if err != nil {
			return 0, err
		}
		return 0, ErrAgentSurfaceAnswered
	}
	msg.SurfaceID = id
	queued, err := s.enqueue(tx, msg, limits)
	if err != nil {
		return 0, err
	}
	return queued, tx.Commit()
}

// ReopenAgentSurface puts a surface whose answer never reached its session
// back in front of the owner, so a dropped answer asks again instead of
// passing for a delivered one.
func (s *Store) ReopenAgentSurface(id string) error {
	_, err := s.db.Exec(`UPDATE agent_surfaces SET answered_at = 0, answer = '' WHERE id = ?`, id)
	return err
}
