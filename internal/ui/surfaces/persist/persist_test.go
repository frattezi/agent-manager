package persist

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/store"
	"github.com/YoanWai/agent-manager/internal/ui/surfaces"
)

func TestTheReadMarksASessionWithAnUnansweredSurface(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	at := time.Unix(1_700_000_000, 0)
	for _, a := range []store.AgentSurface{
		{ID: "a", SessionID: "alpha", PublishedAt: at, Document: "[]"},
		{ID: "b", SessionID: "beta", PublishedAt: at, Document: "[]", AnsweredAt: at.Add(time.Minute), Answer: "done"},
	} {
		if err := st.PublishAgentSurface(a, 10); err != nil {
			t.Fatal(err)
		}
	}
	result, err := surfaces.ReadRequest{}.Run(Services(st))
	if err != nil {
		t.Fatal(err)
	}
	inbox := result.(surfaces.Inbox)
	if current, ok := inbox.Current("alpha"); !ok || current.ID != "a" || current.Document != "[]" {
		t.Fatalf("alpha = %+v, %v", current, ok)
	}
	if inbox.Asking("beta") {
		t.Fatal("an answered surface marks its session")
	}
}

func TestAnAnswerIsRecordedAndQueuedOnTheSessionsInbox(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.PublishAgentSurface(store.AgentSurface{ID: "a", SessionID: "alpha", PublishedAt: time.Now(), Document: "[]"}, 10); err != nil {
		t.Fatal(err)
	}
	request := surfaces.AnswerRequest{SessionID: "alpha", SurfaceID: "a", SurfaceName: "round-1", Answer: "Pick\n  A"}
	result, err := request.Run(Services(st))
	if err != nil {
		t.Fatal(err)
	}
	if result.(surfaces.Inbox).Asking("alpha") {
		t.Fatal("the answered round still asks")
	}
	head, queued, err := st.HeadMessage("alpha")
	if err != nil || !queued || head.SurfaceID != "a" || head.SurfaceName != "round-1" || head.Body != "Pick\n  A" {
		t.Fatalf("queued %v %+v %v", queued, head, err)
	}
	if kept, _ := st.AgentSurface("a"); kept.Answer != "Pick\n  A" {
		t.Fatalf("kept = %+v", kept)
	}
}
