package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YoanWai/agent-manager/internal/sessioncmd"
	"github.com/YoanWai/agent-manager/internal/status"
	"github.com/YoanWai/agent-manager/internal/store"
	uisurfaces "github.com/YoanWai/agent-manager/internal/ui/surfaces"
	uisurfacespersist "github.com/YoanWai/agent-manager/internal/ui/surfaces/persist"
	"github.com/google/uuid"
)

func (w *surfaceWorkspace) open(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(w.configDir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func (w *surfaceWorkspace) answer(t *testing.T, sessionID, id, answer string, flags ...string) (sessioncmd.AgentSurface, error) {
	t.Helper()
	var out bytes.Buffer
	if err := runSurfaceAnswer(&out, strings.NewReader(answer), w.surfaces, append([]string{id, "--json"}, flags...), sessionID); err != nil {
		return sessioncmd.AgentSurface{}, err
	}
	var answered sessioncmd.AgentSurface
	if err := json.Unmarshal(out.Bytes(), &answered); err != nil {
		t.Fatalf("answer --json %q: %v", out.String(), err)
	}
	return answered, nil
}

func TestAnAnswerFromOutsideTheCardClosesTheRound(t *testing.T) {
	w := newSurfaceWorkspace(t)
	published := w.publish(t, surfaceRound(t))
	services := uisurfacespersist.Services(w.open(t))
	if inbox, err := (uisurfaces.ReadRequest{}).Run(services); err != nil || !inbox.(uisurfaces.Inbox).Asking(w.agent.ID) {
		t.Fatalf("before the answer the row is not asking: %v", err)
	}

	var out bytes.Buffer
	if err := runSurfaceAnswer(&out, strings.NewReader("Scope\n  keep it narrow"), w.surfaces, []string{published.ID}, ""); err != nil {
		t.Fatalf("answer: %v", err)
	}
	if want := "answered round-1 (" + published.ID + ") [answered]"; !strings.HasPrefix(out.String(), want) || !strings.Contains(out.String(), "queued for grill-agent") {
		t.Fatalf("answer printed %q", out.String())
	}
	stored := w.stored(t, w.agent.ID)
	if len(stored) != 1 || stored[0].AnsweredAt.IsZero() || stored[0].Answer != "Scope\n  keep it narrow" {
		t.Fatalf("stored = %+v", stored)
	}
	inbox, err := (uisurfaces.ReadRequest{}).Run(services)
	if err != nil || inbox.(uisurfaces.Inbox).Asking(w.agent.ID) {
		t.Fatalf("the row still asks after the answer: %v", err)
	}

	var listed bytes.Buffer
	if err := runSurfaceList(&listed, w.surfaces, []string{w.agent.ID}, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(listed.String(), "[answered]") || !strings.Contains(listed.String(), "  Scope\n    keep it narrow") {
		t.Fatalf("the answer does not read back: %q", listed.String())
	}
}

// The session that asked cannot tell which host collected the answer: the
// message it receives is the one the card queues.
func TestAnAnswerFromOutsideReachesTheSessionAsTheCardsDoes(t *testing.T) {
	w := newSurfaceWorkspace(t)
	st := w.open(t)
	other := store.Session{ID: uuid.NewString()[:8], Name: "grill-2", Tool: "claude", Cwd: w.configDir, Status: status.Idle}
	if err := st.CreateSession(other); err != nil {
		t.Fatal(err)
	}
	fromHost := w.publish(t, surfaceRound(t))
	var out bytes.Buffer
	if err := runSurfacePublish(&out, bytes.NewReader(surfaceRound(t)), w.surfaces, []string{"--json"}, other.ID); err != nil {
		t.Fatal(err)
	}
	var fromCard sessioncmd.AgentSurface
	if err := json.Unmarshal(out.Bytes(), &fromCard); err != nil {
		t.Fatal(err)
	}

	if _, err := w.answer(t, "", fromHost.ID, "the same answer"); err != nil {
		t.Fatal(err)
	}
	if _, err := (uisurfaces.AnswerRequest{SessionID: other.ID, SurfaceID: fromCard.ID, SurfaceName: "round-1", Answer: "the same answer"}).
		Run(uisurfacespersist.Services(st)); err != nil {
		t.Fatal(err)
	}

	host, ok, err := st.HeadMessage(w.agent.ID)
	if err != nil || !ok {
		t.Fatalf("nothing queued for the asking session: %v", err)
	}
	card, ok, err := st.HeadMessage(other.ID)
	if err != nil || !ok {
		t.Fatalf("nothing queued by the card: %v", err)
	}
	if host.SurfaceID != fromHost.ID || host.Fingerprint != "surface:"+fromHost.ID {
		t.Fatalf("host message names %q (%q), want %q", host.SurfaceID, host.Fingerprint, fromHost.ID)
	}
	if host.SenderName != card.SenderName || host.SenderID != card.SenderID || host.Body != card.Body || host.SurfaceName != card.SurfaceName {
		t.Fatalf("the host queued\n%+v\nthe card queued\n%+v", host, card)
	}
}

func TestAnAnswerThatCannotBeRecordedIsRefusedWithTheReason(t *testing.T) {
	w := newSurfaceWorkspace(t)
	published := w.publish(t, surfaceRound(t))

	if _, err := w.answer(t, "", "nosuch12", "yes"); err == nil || !strings.Contains(err.Error(), "surface nosuch12 does not exist") {
		t.Fatalf("answer to an unknown surface = %v", err)
	}
	if _, err := w.answer(t, w.agent.ID, published.ID, "yes"); err == nil || !strings.Contains(err.Error(), "only the user answers it") {
		t.Fatalf("the asking session answering itself = %v", err)
	}
	if _, err := w.answer(t, "", published.ID, " \n"); err == nil || !strings.Contains(err.Error(), "the answer is empty") {
		t.Fatalf("an empty answer = %v", err)
	}
	if got := w.stored(t, w.agent.ID); !got[0].AnsweredAt.IsZero() {
		t.Fatalf("a refused answer was recorded: %+v", got)
	}

	if _, err := w.answer(t, "", published.ID, "first"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.answer(t, "", published.ID, "second"); err == nil || !strings.Contains(err.Error(), "already answered") {
		t.Fatalf("a second answer = %v", err)
	}
	if got := w.stored(t, w.agent.ID); got[0].Answer != "first" {
		t.Fatalf("the second answer replaced the first: %+v", got)
	}
	if queued, err := w.open(t).QueuedCount(w.agent.ID); err != nil || queued != 1 {
		t.Fatalf("queued %d answers (%v), want 1", queued, err)
	}
}

func TestADeliveredAnswerClosesTheRoundWithoutQueuingIt(t *testing.T) {
	w := newSurfaceWorkspace(t)
	published := w.publish(t, surfaceRound(t))
	st := w.open(t)

	if _, err := w.answer(t, "", published.ID, "yes", "--delivered"); err == nil || !strings.Contains(err.Error(), "never reach that conversation") {
		t.Fatalf("delivered from outside = %v", err)
	}
	if _, err := w.answer(t, w.agent.ID, published.ID, "yes"); err == nil || !strings.Contains(err.Error(), "only the user answers it") {
		t.Fatalf("undelivered from the asker = %v", err)
	}
	if _, err := w.answer(t, w.agent.ID, published.ID, " ", "--delivered"); err == nil || !strings.Contains(err.Error(), "the answer is empty") {
		t.Fatalf("empty delivered answer = %v", err)
	}
	if _, err := w.answer(t, w.agent.ID, "nosuch12", "yes", "--delivered"); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("delivered to an unknown surface = %v", err)
	}

	if _, err := w.answer(t, w.agent.ID, published.ID, "keep it narrow", "--delivered"); err != nil {
		t.Fatalf("delivered answer: %v", err)
	}
	if got := w.stored(t, w.agent.ID); got[0].AnsweredAt.IsZero() || got[0].Answer != "keep it narrow" {
		t.Fatalf("stored = %+v", got)
	}
	inbox, err := (uisurfaces.ReadRequest{}).Run(uisurfacespersist.Services(st))
	if err != nil || inbox.(uisurfaces.Inbox).Asking(w.agent.ID) {
		t.Fatalf("the row still asks: %v", err)
	}
	if _, ok, err := st.HeadMessage(w.agent.ID); err != nil || ok {
		t.Fatalf("a delivered answer was queued: %v", err)
	}
	if _, err := w.answer(t, w.agent.ID, published.ID, "again", "--delivered"); err == nil || !strings.Contains(err.Error(), "already answered") {
		t.Fatalf("second delivered answer = %v", err)
	}
}
