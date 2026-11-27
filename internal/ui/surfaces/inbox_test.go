package surfaces

import (
	"errors"
	"testing"
	"time"
)

func TestTheInboxKeepsEachSessionsNewestUnansweredSurface(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	inbox := NewInbox([]Stored{
		{ID: "a1", SessionID: "alpha", PublishedAt: base},
		{ID: "a2", SessionID: "alpha", PublishedAt: base.Add(time.Second)},
		{ID: "b1", SessionID: "beta", PublishedAt: base, AnsweredAt: base.Add(time.Minute)},
	})
	if current, ok := inbox.Current("alpha"); !ok || current.ID != "a2" {
		t.Fatalf("alpha current = %+v, %v; want a2", current, ok)
	}
	if inbox.Asking("beta") {
		t.Fatal("an answered surface still asks")
	}
	if inbox.Asking("gamma") {
		t.Fatal("a session with no surface asks")
	}
	if (Inbox{}).Asking("alpha") {
		t.Fatal("the empty inbox asks")
	}
}

// A newer answered surface replaces an older unanswered one: the round the
// session is on is the newest.
func TestAnAnsweredNewerSurfaceLeavesTheSessionAskingNothing(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	inbox := NewInbox([]Stored{
		{ID: "new", SessionID: "alpha", PublishedAt: base.Add(time.Second), AnsweredAt: base.Add(time.Minute)},
		{ID: "old", SessionID: "alpha", PublishedAt: base},
	})
	if inbox.Asking("alpha") {
		t.Fatal("an older round asks past a newer answered one")
	}
}

type fakeStore struct {
	latest []Stored
	err    error
}

func (f fakeStore) LatestSurfaces() ([]Stored, error) { return f.latest, f.err }

func TestReadRequestReadsTheInboxFromTheStore(t *testing.T) {
	result, err := ReadRequest{}.Run(Services{Store: fakeStore{latest: []Stored{{ID: "s", SessionID: "alpha"}}}})
	if err != nil || !result.(Inbox).Asking("alpha") {
		t.Fatalf("read = %+v, %v", result, err)
	}
	broken := errors.New("disk")
	if _, err := (ReadRequest{}).Run(Services{Store: fakeStore{err: broken}}); !errors.Is(err, broken) {
		t.Fatalf("err = %v", err)
	}
}
