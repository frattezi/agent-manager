package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/store"
	uisurfaces "github.com/YoanWai/agent-manager/internal/ui/surfaces"
	tea "github.com/charmbracelet/bubbletea"
)

// sendFromCard opens alpha's round, overrides the second question and
// sends it with ↵, draining the effect lane the answer rides.
func sendFromCard(t *testing.T) (*Model, string) {
	t.Helper()
	m, card := openTestCard(t)
	for _, k := range []tea.KeyMsg{{Type: tea.KeyTab}, {Type: tea.KeyRight}, {Type: tea.KeyEnter}} {
		m.handleKey(k)
	}
	m.drainEffects(t)
	return m, card.SessionID()
}

func TestSendingQueuesTheAnswerOnTheSessionsInboxAndClearsTheMark(t *testing.T) {
	m, id := sendFromCard(t)
	if _, open := surfaceOf[*uisurfaces.Card](m); open {
		t.Fatalf("the card stayed open after sending; err %q", m.errBar.text)
	}
	head, queued, err := m.services.store.HeadMessage(id)
	if err != nil || !queued {
		t.Fatalf("nothing queued: %v", err)
	}
	if head.SurfaceID != "round-1" || head.SurfaceName != "round-1" || head.SenderID != "" {
		t.Fatalf("the queued answer does not name its round: %+v", head)
	}
	if !strings.Contains(head.Body, "All of them [all] (overriding the recommended: A named cap [cap])") {
		t.Fatalf("body = %q", head.Body)
	}
	// The answer goes through the inbox, which waits for the pane to rest;
	// the send itself types nothing.
	pane, err := m.services.tmux.CapturePane(id)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(pane, "All of them") {
		t.Fatalf("the send wrote to the pane:\n%s", pane)
	}
	if m.workspace.surfaces.inbox.Asking(id) {
		t.Fatal("the answered round still marks its row")
	}
	if row := railRowText(t, m, "alpha"); strings.Contains(row, "alpha "+m.glyphs().AskingGlyph) {
		t.Fatalf("alpha's row = %q", row)
	}
}

func TestAnAnsweredRoundReadsBackWithItsAnswerInTheOrderAsked(t *testing.T) {
	m, id := sendFromCard(t)
	later := store.AgentSurface{ID: "round-2", SessionID: id, PublishedAt: time.Now().Add(time.Second), Document: roundDocument(t)}
	if err := m.services.store.PublishAgentSurface(later, 50); err != nil {
		t.Fatal(err)
	}
	rounds, err := m.services.store.AgentSurfaces(id)
	if err != nil || len(rounds) != 2 {
		t.Fatalf("rounds = %+v, %v", rounds, err)
	}
	if rounds[0].ID != "round-1" || rounds[0].AnsweredAt.IsZero() || !strings.Contains(rounds[0].Answer, "All of them") {
		t.Fatalf("the first round = %+v", rounds[0])
	}
	if rounds[1].ID != "round-2" || !rounds[1].AnsweredAt.IsZero() {
		t.Fatalf("the second round = %+v", rounds[1])
	}
}

func TestClickingSendOnTheCardsFootSendsTheRound(t *testing.T) {
	m, card := openTestCard(t)
	preparedView(m)
	x, y := paintedCell(t, m, "send")
	m = leftClick(m, x, y)
	m.drainEffects(t)
	if _, queued, _ := m.services.store.HeadMessage(card.SessionID()); !queued {
		t.Fatalf("the click queued nothing; err %q", m.errBar.text)
	}
}

// A refused answer is reported, leaves the round unanswered and keeps the
// card open on what the owner chose.
func TestARefusedAnswerKeepsTheCardOpenAndSaysWhy(t *testing.T) {
	m, card := openTestCard(t)
	for range store.DefaultInboxLimits.QueueCap {
		if _, err := m.services.store.Enqueue(store.InboxMessage{
			SessionID: card.SessionID(), SenderID: "peer", Body: time.Now().String(), Fingerprint: time.Now().String(), SentAt: time.Now(),
		}, store.InboxLimits{QueueCap: 100, RateCap: 100, RateWindow: time.Minute, DedupeWindow: time.Nanosecond}); err != nil {
			t.Fatal(err)
		}
	}
	m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m.drainEffects(t)
	if top, _ := m.top(); top != card {
		t.Fatal("a refused answer closed the card")
	}
	if !strings.Contains(m.errBar.text, "was not sent") || !strings.Contains(m.errBar.text, "queue is full") {
		t.Fatalf("err = %q", m.errBar.text)
	}
	if kept, _ := m.services.store.AgentSurface("round-1"); !kept.AnsweredAt.IsZero() {
		t.Fatalf("a refused answer marked the round answered: %+v", kept)
	}
	if !m.workspace.surfaces.inbox.Asking(card.SessionID()) {
		t.Fatal("the mark cleared on a refusal")
	}
}

// Nothing but the owner's send answers a round: closing the card, and the
// polls that follow, queue nothing.
func TestARoundNeverAnswersItself(t *testing.T) {
	m, card := openTestCard(t)
	m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	for range 3 {
		m.applyTestMsg(t, surfacesReadNow())
		m.drainEffects(t)
	}
	if _, queued, _ := m.services.store.HeadMessage(card.SessionID()); queued {
		t.Fatal("a closed round sent an answer")
	}
	if kept, _ := m.services.store.AgentSurface("round-1"); !kept.AnsweredAt.IsZero() {
		t.Fatalf("the round answered itself: %+v", kept)
	}
}
