package store

import (
	"testing"
	"time"
)

func TestASessionMarkIsClearedOnlyByTheProcessThatSetIt(t *testing.T) {
	st := newTestStore(t)
	at := time.Unix(1_700_000_000, 0)
	for _, pid := range []int{100, 200} {
		if err := st.SetSessionMark(SessionMark{SessionID: "a1", Name: MarkBabysit, Pid: pid, SetAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.ClearSessionMark("a1", MarkBabysit, 100); err != nil {
		t.Fatal(err)
	}
	marks, err := st.SessionMarks()
	if err != nil {
		t.Fatal(err)
	}
	if len(marks) != 1 || marks[0].Pid != 200 || marks[0].SessionID != "a1" || !marks[0].SetAt.Equal(at) {
		t.Fatalf("want only pid 200's mark left, got %+v", marks)
	}
}

func TestOnlyBabysitIsAKnownSessionMark(t *testing.T) {
	if !KnownSessionMark(MarkBabysit) || KnownSessionMark("deploying") {
		t.Fatal("babysit is the only mark the rail can draw")
	}
}
