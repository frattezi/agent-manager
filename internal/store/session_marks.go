package store

import (
	"slices"
	"time"
)

// MarkBabysit is the Session Mark of a session driving Fractory Tickets to
// merged with `fractory babysit`.
const MarkBabysit = "babysit"

// SessionMarkNames are the only marks a session may set: each one has a
// glyph the rail draws, so a name outside the list would draw nothing.
var SessionMarkNames = []string{MarkBabysit}

// KnownSessionMark reports whether name is one of SessionMarkNames.
func KnownSessionMark(name string) bool { return slices.Contains(SessionMarkNames, name) }

// SessionMark is one named activity a session's row shows while the process
// that set it, Pid, is still running.
type SessionMark struct {
	SessionID string
	Name      string
	Pid       int
	SetAt     time.Time
}

func (s *Store) SetSessionMark(m SessionMark) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO session_marks (session_id, name, pid, set_at) VALUES (?, ?, ?, ?)`,
		m.SessionID, m.Name, m.Pid, encodeTime(m.SetAt))
	return err
}

// ClearSessionMark removes the mark one process set, leaving any other
// process's mark of the same name on the session.
func (s *Store) ClearSessionMark(sessionID, name string, pid int) error {
	_, err := s.db.Exec(`DELETE FROM session_marks WHERE session_id = ? AND name = ? AND pid = ?`, sessionID, name, pid)
	return err
}

// SessionMarks lists every stored mark, whether or not its process still
// runs; the reader decides that.
func (s *Store) SessionMarks() ([]SessionMark, error) {
	rows, err := s.db.Query(`SELECT session_id, name, pid, set_at FROM session_marks ORDER BY set_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var marks []SessionMark
	for rows.Next() {
		var m SessionMark
		var setAt int64
		if err := rows.Scan(&m.SessionID, &m.Name, &m.Pid, &setAt); err != nil {
			return nil, err
		}
		m.SetAt = decodeTime(setAt)
		marks = append(marks, m)
	}
	return marks, rows.Err()
}
