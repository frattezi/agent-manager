package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YoanWai/agent-manager/internal/sessioncmd"
	"github.com/YoanWai/agent-manager/internal/status"
	"github.com/YoanWai/agent-manager/internal/store"
	"github.com/YoanWai/agent-manager/internal/ui/a2ui"
	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	"github.com/google/uuid"
)

// surfaceRound is the grilling round the format's own tests render.
func surfaceRound(t *testing.T) []byte {
	t.Helper()
	doc, err := os.ReadFile(filepath.Join("..", "ui", "a2ui", "testdata", "round.json"))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

type surfaceWorkspace struct {
	configDir string
	surfaces  surfaceCommands
	agent     store.Session
	terminal  store.Session
}

func newSurfaceWorkspace(t *testing.T) *surfaceWorkspace {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	configDir := t.TempDir()
	st, err := store.Open(filepath.Join(configDir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	w := &surfaceWorkspace{
		configDir: configDir,
		surfaces:  sessioncmd.NewSessions(configDir, sessioncmd.CLIVocabulary()),
		agent:     store.Session{ID: uuid.NewString()[:8], Name: "grill-agent", Tool: "claude", Cwd: configDir, Status: status.Idle},
		terminal:  store.Session{ID: uuid.NewString()[:8], Name: "shell-1", Tool: "terminal", Cwd: configDir, Status: status.Idle},
	}
	for _, sess := range []store.Session{w.agent, w.terminal} {
		if err := st.CreateSession(sess); err != nil {
			t.Fatal(err)
		}
	}
	return w
}

func (w *surfaceWorkspace) stored(t *testing.T, sessionID string) []store.AgentSurface {
	t.Helper()
	st, err := store.Open(filepath.Join(w.configDir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	surfaces, err := st.AgentSurfaces(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	return surfaces
}

func (w *surfaceWorkspace) publish(t *testing.T, input []byte, args ...string) sessioncmd.AgentSurface {
	t.Helper()
	var out bytes.Buffer
	if err := runSurfacePublish(&out, bytes.NewReader(input), w.surfaces, append(args, "--json"), w.agent.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}
	var published sessioncmd.AgentSurface
	if err := json.Unmarshal(out.Bytes(), &published); err != nil {
		t.Fatalf("publish --json %q: %v", out.String(), err)
	}
	return published
}

func TestASessionPublishesASurfaceFromStdin(t *testing.T) {
	w := newSurfaceWorkspace(t)
	doc := surfaceRound(t)
	var out bytes.Buffer
	if err := runSurfacePublish(&out, bytes.NewReader(doc), w.surfaces, nil, w.agent.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !strings.HasPrefix(out.String(), "published round-1 (") || !strings.Contains(out.String(), "[unanswered]") || !strings.Contains(out.String(), "for grill-agent") {
		t.Fatalf("publish printed %q", out.String())
	}
	stored := w.stored(t, w.agent.ID)
	if len(stored) != 1 || stored[0].Document != string(doc) {
		t.Fatalf("stored = %+v", stored)
	}
}

func TestASessionPublishesASurfaceFromAFile(t *testing.T) {
	w := newSurfaceWorkspace(t)
	path := filepath.Join(t.TempDir(), "round.json")
	if err := os.WriteFile(path, surfaceRound(t), 0o644); err != nil {
		t.Fatal(err)
	}
	published := w.publish(t, nil, path)
	if published.SessionID != w.agent.ID || published.SurfaceID != "round-1" || published.Answered {
		t.Fatalf("published = %+v", published)
	}
	if got := w.stored(t, w.agent.ID); len(got) != 1 || got[0].ID != published.ID {
		t.Fatalf("stored = %+v", got)
	}
}

func TestPublishingOutsideASessionIsRefused(t *testing.T) {
	w := newSurfaceWorkspace(t)
	err := runSurfacePublish(&bytes.Buffer{}, bytes.NewReader(surfaceRound(t)), w.surfaces, nil, "")
	if err == nil || !strings.Contains(err.Error(), "needs one") || !strings.Contains(err.Error(), "not inside an Agent Manager session") {
		t.Fatalf("publish with no caller = %v", err)
	}
}

func TestPublishingAsATerminalIsRefused(t *testing.T) {
	w := newSurfaceWorkspace(t)
	err := runSurfacePublish(&bytes.Buffer{}, bytes.NewReader(surfaceRound(t)), w.surfaces, nil, w.terminal.ID)
	if err == nil || !strings.Contains(err.Error(), "shell-1 is a terminal") || !strings.Contains(err.Error(), "no conversation to return to") {
		t.Fatalf("publish as a terminal = %v", err)
	}
	if got := w.stored(t, w.terminal.ID); len(got) != 0 {
		t.Fatalf("a terminal's surface was stored: %+v", got)
	}
}

// The refusal is what the agent fixes its document from, so it names the
// field, or the version, at fault.
func TestABadDocumentIsRefusedWithWhatIsWrongAndNothingStored(t *testing.T) {
	w := newSurfaceWorkspace(t)
	for _, c := range []struct {
		name, doc, want string
	}{
		{"not JSON", `{`, "document: must be a JSON array"},
		{"wrong version", `[{"version":"v0.8","createSurface":{"surfaceId":"r"}}]`, `messages[0].version is "v0.8"; this build reads A2UI "v0.9"`},
		{"missing field", `[{"version":"v0.9","createSurface":{}}]`, "messages[0].createSurface.surfaceId"},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := runSurfacePublish(&bytes.Buffer{}, strings.NewReader(c.doc), w.surfaces, nil, w.agent.ID)
			if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "nothing was stored") {
				t.Fatalf("publish %s = %v, want it to name %q", c.name, err, c.want)
			}
		})
	}
	if err := runSurfacePublish(&bytes.Buffer{}, strings.NewReader(""), w.surfaces, nil, w.agent.ID); err == nil || !strings.Contains(err.Error(), "document is empty") {
		t.Fatalf("empty publish = %v", err)
	}
	if got := w.stored(t, w.agent.ID); len(got) != 0 {
		t.Fatalf("a refused document was stored: %+v", got)
	}
}

func TestAStoredSurfaceIsListedAndDrawnAsText(t *testing.T) {
	w := newSurfaceWorkspace(t)
	doc := surfaceRound(t)
	published := w.publish(t, doc)

	var listed bytes.Buffer
	if err := runSurfaceList(&listed, w.surfaces, nil, w.agent.ID); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(listed.String(), "round-1 ("+published.ID+") [unanswered]") {
		t.Fatalf("own list = %q", listed.String())
	}
	// The owner reads a session's rounds from a shell no session runs.
	var fromOutside bytes.Buffer
	if err := runSurfaceList(&fromOutside, w.surfaces, []string{w.agent.ID, "--json"}, ""); err != nil {
		t.Fatal(err)
	}
	var records []sessioncmd.AgentSurface
	if err := json.Unmarshal(fromOutside.Bytes(), &records); err != nil || len(records) != 1 || records[0].ID != published.ID {
		t.Fatalf("list --json = %q (%v)", fromOutside.String(), err)
	}
	if err := runSurfaceList(&bytes.Buffer{}, w.surfaces, nil, ""); err == nil || !strings.Contains(err.Error(), "name the session") {
		t.Fatalf("list naming nobody from outside = %v", err)
	}

	parsed, err := a2ui.Parse(doc)
	if err != nil {
		t.Fatal(err)
	}
	var shown bytes.Buffer
	if err := runSurfaceShow(&shown, w.surfaces, []string{published.ID, "--width", "60"}, ""); err != nil {
		t.Fatal(err)
	}
	if want := strings.TrimRight(a2ui.Render(parsed, 60, presentation.ChromeFor(presentation.DefaultGlyphMode)), "\n") + "\n"; shown.String() != want {
		t.Fatalf("show =\n%s\nwant\n%s", shown.String(), want)
	}
	if strings.Contains(shown.String(), "\x1b") {
		t.Fatalf("show wrote escapes:\n%q", shown.String())
	}
	if err := runSurfaceShow(&bytes.Buffer{}, w.surfaces, []string{"nosuch12"}, ""); err == nil || !strings.Contains(err.Error(), "agent-manager surface list") {
		t.Fatalf("show of a missing surface = %v", err)
	}
}

func TestPublishingPastTheRetentionCapDropsTheOldest(t *testing.T) {
	w := newSurfaceWorkspace(t)
	doc := surfaceRound(t)
	first := w.publish(t, doc)
	var last sessioncmd.AgentSurface
	for range sessioncmd.RetainedSurfaces {
		last = w.publish(t, doc)
	}
	stored := w.stored(t, w.agent.ID)
	if len(stored) != sessioncmd.RetainedSurfaces {
		t.Fatalf("kept %d surfaces, want %d", len(stored), sessioncmd.RetainedSurfaces)
	}
	for _, s := range stored {
		if s.ID == first.ID {
			t.Fatal("the oldest surface survived the cap")
		}
	}
	if stored[len(stored)-1].ID != last.ID {
		t.Fatalf("newest kept = %s, want %s", stored[len(stored)-1].ID, last.ID)
	}
}

func TestDeletingASessionRemovesItsSurfaces(t *testing.T) {
	w := newSurfaceWorkspace(t)
	w.publish(t, surfaceRound(t))
	st, err := store.Open(filepath.Join(w.configDir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Delete(w.agent.ID); err != nil {
		t.Fatal(err)
	}
	st.Close()
	if got := w.stored(t, w.agent.ID); len(got) != 0 {
		t.Fatalf("surfaces outlived their session: %+v", got)
	}
}
