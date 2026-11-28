package sessioncmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/YoanWai/agent-manager/internal/conversation"

	"github.com/YoanWai/agent-manager/internal/store"
)

// SetMark puts a Session Mark on the calling session for the process pid,
// which the manager drops once pid is gone even if nothing clears it.
func (s *Sessions) SetMark(sessionID, name string, pid int) error {
	if !store.KnownSessionMark(name) {
		return fmt.Errorf("%q is not a mark the rail draws; it takes %s", name, strings.Join(store.SessionMarkNames, ", "))
	}
	runtime, err := s.open()
	if err != nil {
		return err
	}
	defer runtime.Close()
	caller, err := runtime.caller(sessionID)
	if err != nil {
		return fmt.Errorf("a mark goes on the session that runs the command, so it needs one: %w", err)
	}
	if err := pruneDeadMarks(runtime.store); err != nil {
		return err
	}
	return runtime.store.SetSessionMark(store.SessionMark{SessionID: caller.ID, Name: name, Pid: pid, SetAt: time.Now()})
}

// pruneDeadMarks removes the marks a killed wrapper left behind, so they
// never pile up or come back on a reused pid.
func pruneDeadMarks(st *store.Store) error {
	marks, err := st.SessionMarks()
	if err != nil {
		return err
	}
	for _, m := range marks {
		if !conversation.Alive(m.Pid) {
			if err := st.ClearSessionMark(m.SessionID, m.Name, m.Pid); err != nil {
				return err
			}
		}
	}
	return nil
}

// ClearMark removes the mark pid set on the calling session.
func (s *Sessions) ClearMark(sessionID, name string, pid int) error {
	runtime, err := s.open()
	if err != nil {
		return err
	}
	defer runtime.Close()
	return runtime.store.ClearSessionMark(sessionID, name, pid)
}
