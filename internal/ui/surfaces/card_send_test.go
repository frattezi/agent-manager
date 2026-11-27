package surfaces

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

var enter = tea.KeyMsg{Type: tea.KeyEnter}

func openSending(t *testing.T) (*Card, *fakeHost, *[]AnswerRequest) {
	t.Helper()
	sent := &[]AnswerRequest{}
	card, err := Open(fakeRoot{sent: sent}, Stored{ID: "st-1", SessionID: "alpha", Document: roundDocument(t)}, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	return card, &fakeHost{cols: 100, rows: 40}, sent
}

func TestEnterSendsTheRoundAsAnswered(t *testing.T) {
	card, h, sent := openSending(t)
	press(card, h, tab, right, tab, runes("keep it short"), enter)
	if len(*sent) != 1 {
		t.Fatalf("sent %d answers", len(*sent))
	}
	got := (*sent)[0]
	if got.SessionID != "alpha" || got.SurfaceID != "st-1" || got.SurfaceName != "round-1" {
		t.Fatalf("request = %+v", got)
	}
	for _, want := range []string{
		"internal/ui/a2ui, boundary-restricted like internal/ui/runs [a2ui] (the recommended answer)",
		"All of them [all] (overriding the recommended: A named cap [cap])",
		"keep it short",
	} {
		if !strings.Contains(got.Answer, want) {
			t.Errorf("answer lacks %q:\n%s", want, got.Answer)
		}
	}
	if h.popped != 0 {
		t.Fatal("the card closed before the answer was queued")
	}
}

func TestASecondSendWaitsForTheFirstToBeRefused(t *testing.T) {
	card, h, sent := openSending(t)
	press(card, h, enter, enter)
	if len(*sent) != 1 {
		t.Fatalf("two presses sent %d answers", len(*sent))
	}
	card.Refused()
	press(card, h, enter)
	if len(*sent) != 2 {
		t.Fatalf("after a refusal the round cannot be sent again: %d", len(*sent))
	}
}

func TestClosingTheCardSendsNothing(t *testing.T) {
	card, h, sent := openSending(t)
	press(card, h, tab, right, esc)
	card.Leave(h)
	if len(*sent) != 0 {
		t.Fatalf("closing sent %+v", *sent)
	}
}

func TestTheFootNamesTheSendKey(t *testing.T) {
	card, h, _ := openSending(t)
	card.View(h, "")
	if !hinted(h.hint, "↵") {
		t.Fatalf("hint = %v", h.hint)
	}
}
