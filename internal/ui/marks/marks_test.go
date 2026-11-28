package marks

import (
	"errors"
	"testing"
)

type fakeStore struct {
	stored []Stored
	err    error
}

func (f fakeStore) SessionMarks() ([]Stored, error) { return f.stored, f.err }

func TestAMarkWhoseProcessIsGoneIsDropped(t *testing.T) {
	stored := []Stored{{SessionID: "a1", Name: "babysit", Pid: 10}, {SessionID: "b2", Name: "babysit", Pid: 20}}
	result, err := ReadRequest{}.Run(Services{Store: fakeStore{stored: stored}, Alive: func(pid int) bool { return pid == 10 }})
	if err != nil {
		t.Fatal(err)
	}
	set := result.(Set)
	if !set.Has("a1", "babysit") || set.Has("b2", "babysit") || set.Has("a1", "other") {
		t.Fatalf("want only a1's live babysit mark, got %+v", set)
	}
}

func TestAFailedReadMarksNothing(t *testing.T) {
	result, err := ReadRequest{}.Run(Services{Store: fakeStore{err: errors.New("locked")}, Alive: func(int) bool { return true }})
	if err == nil || result.(Set).Has("a1", "babysit") {
		t.Fatalf("want the error and an empty set, got %v %+v", err, result)
	}
}
