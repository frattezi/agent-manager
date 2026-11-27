package execution

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/status"
	"github.com/YoanWai/agent-manager/internal/store"
)

// queueAnswer answers a surface the session published the way the card's
// send does: recorded on the surface and queued on the inbox.
func queueAnswer(t *testing.T, m *harness, sessionID, surfaceID, body string) {
	t.Helper()
	if err := m.store.PublishAgentSurface(store.AgentSurface{ID: surfaceID, SessionID: sessionID, PublishedAt: time.Now(), Document: "[]"}, 50); err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.AnswerAgentSurface(surfaceID, body, store.InboxMessage{
		SessionID: sessionID, SenderName: "the user", Body: body, Fingerprint: "surface:" + surfaceID,
		SentAt: time.Now(), SurfaceName: "round-1",
	}, store.DefaultInboxLimits); err != nil {
		t.Fatal(err)
	}
}

var answerFence = regexp.MustCompile(`-{4}SURFACE-ANSWER-round-1-[A-Z2-7]{8}-{4}`)

func TestTheOwnersAnswerIsFencedAsTheirAnswerToTheRoundItNames(t *testing.T) {
	msg := store.InboxMessage{
		SenderName:  "the user",
		Body:        "1. Which package?\n  a2ui (the recommended answer)\x1b[31m",
		SentAt:      time.Date(2026, 11, 27, 9, 30, 0, 0, time.Local),
		SurfaceID:   "ab12cd34",
		SurfaceName: "round-1",
	}
	envelope := inboxEnvelope(msg, "claude", false)
	fence := answerFence.FindString(envelope)
	if fence == "" {
		t.Fatalf("no answer fence: %q", envelope)
	}
	lines := strings.Split(envelope, "\n")
	var at []int
	for i, line := range lines {
		if line == fence {
			at = append(at, i)
		}
	}
	if len(at) != 2 {
		t.Fatalf("the fence delimits %d times:\n%s", len(at), envelope)
	}
	if body := strings.Join(lines[at[0]+1:at[1]], "\n"); body != "1. Which package?\n  a2ui (the recommended answer)[31m" {
		t.Fatalf("fenced body = %q", body)
	}
	for _, want := range []string{"The user's answer to the round you published", `surface ab12cd34 (surfaceId "round-1")`} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("the preamble lacks %q: %q", want, lines[0])
		}
	}
	if strings.Contains(envelope, "CROSS-SESSION-MESSAGE") || strings.Contains(envelope, "reply with") {
		t.Fatalf("the answer reads as another agent's message: %q", envelope)
	}
	if again := answerFence.FindString(inboxEnvelope(msg, "claude", false)); again == fence {
		t.Fatal("the fence repeats across deliveries")
	}
}

func TestAnAnswerWaitsForThePaneToRestAndIsTypedOnce(t *testing.T) {
	m := buildModel(t)
	sess := spawnedSession(t, m, "claude-hooked")
	queueAnswer(t, m, sess.ID, "s1", "Ship it\n  checked (the recommended answer)")

	if delivered, err := m.poller.maybeDeliverInbox(sess, "❯ ", status.Working, true); err != nil || delivered {
		t.Fatalf("delivered mid-turn: %v, %v", delivered, err)
	}
	if delivered, err := m.poller.maybeDeliverInbox(sess, "❯ ", status.Idle, true); err != nil || !delivered {
		t.Fatalf("not delivered at rest: %v, %v", delivered, err)
	}
	settledPane(t, m, sess.ID, "SURFACE-ANSWER-round-1", "checked (the recommended answer)")
	if delivered, err := m.poller.maybeDeliverInbox(sess, "❯ ", status.Idle, true); err != nil || delivered {
		t.Fatalf("typed twice: %v, %v", delivered, err)
	}
	if kept, _ := m.store.AgentSurface("s1"); kept.AnsweredAt.IsZero() || !strings.Contains(kept.Answer, "Ship it") {
		t.Fatalf("the delivered answer was not kept: %+v", kept)
	}
}

// An answer that never reached its session is reported to the owner and
// the round asks again, rather than passing for answered.
func TestAnAnswerThatCannotBeTypedIsReportedAndTheRoundReopens(t *testing.T) {
	m := buildModel(t)
	sess := spawnedSession(t, m, "claude-hooked")
	queueAnswer(t, m, sess.ID, "s1", "Ship it")
	if err := m.tmux.Kill(sess.ID); err != nil {
		t.Fatal(err)
	}
	_, err := m.poller.maybeDeliverInbox(sess, "❯ ", status.Idle, true)
	if err == nil || !strings.Contains(err.Error(), "your answer to worker's round s1 did not reach it") {
		t.Fatalf("a failed answer reported %v", err)
	}
	if kept, _ := m.store.AgentSurface("s1"); !kept.AnsweredAt.IsZero() {
		t.Fatalf("a dropped answer left the round answered: %+v", kept)
	}
	if queued, _ := m.store.QueuedCount(sess.ID); queued != 0 {
		t.Fatal("a dropped answer was left to be retried")
	}
}
