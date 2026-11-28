// Package marks is what the manager keeps of its sessions' Session Marks:
// which named activity each session's row shows, as the last poll found
// them. A mark counts only while the process that set it still runs.
package marks

// Stored is one mark as the store keeps it.
type Stored struct {
	SessionID string
	Name      string
	Pid       int
}

// Store is where the poll reads every stored mark.
type Store interface {
	SessionMarks() ([]Stored, error)
}

// Services is what the poll runs against; Alive reports whether a pid
// still runs, the check that drops a mark whose wrapper died uncleared.
type Services struct {
	Store Store
	Alive func(pid int) bool
}

// ReadRequest reads the marks of every session.
type ReadRequest struct{}

func (ReadRequest) EffectName() string { return "Session Marks read" }

func (ReadRequest) Run(services Services) (any, error) {
	stored, err := services.Store.SessionMarks()
	if err != nil {
		return Set{}, err
	}
	return NewSet(stored, services.Alive), nil
}

// Set is each session's live marks.
type Set struct {
	live map[string]map[string]bool
}

// NewSet keeps the marks whose process alive reports running.
func NewSet(stored []Stored, alive func(int) bool) Set {
	set := Set{live: map[string]map[string]bool{}}
	for _, m := range stored {
		if !alive(m.Pid) {
			continue
		}
		if set.live[m.SessionID] == nil {
			set.live[m.SessionID] = map[string]bool{}
		}
		set.live[m.SessionID][m.Name] = true
	}
	return set
}

// Has reports whether the session wears the named mark.
func (s Set) Has(sessionID, name string) bool { return s.live[sessionID][name] }
