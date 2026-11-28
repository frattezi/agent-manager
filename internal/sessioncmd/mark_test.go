package sessioncmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/store"
)

func TestANewMarkPrunesTheMarksKilledWrappersLeft(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, pid := range []int{os.Getpid(), 1 << 30} {
		if err := st.SetSessionMark(store.SessionMark{SessionID: "a1", Name: store.MarkBabysit, Pid: pid, SetAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	if err := pruneDeadMarks(st); err != nil {
		t.Fatal(err)
	}
	marks, err := st.SessionMarks()
	if err != nil {
		t.Fatal(err)
	}
	if len(marks) != 1 || marks[0].Pid != os.Getpid() {
		t.Fatalf("want only the live wrapper's mark, got %+v", marks)
	}
}

func TestSetMarkRefusesANameTheRailCannotDraw(t *testing.T) {
	if err := NewSessions(t.TempDir(), CLIVocabulary()).SetMark("cafe0001", "deploying", 1); err == nil {
		t.Fatal("want an unknown mark refused")
	}
}
