package mcpserver

import (
	"strings"
	"testing"
)

func TestSurfaceForwardsEachActionToTheCommandLayer(t *testing.T) {
	fake := &fakeSessionCommands{}
	session := connectServer(t, serverWithFakes(t, fake))

	if text, isError := callText(t, session, "surface", map[string]any{"action": "publish", "document": `[{"version":"v0.9"}]`}); isError ||
		!strings.Contains(text, "published round-1 (s1)") {
		t.Fatalf("publish = %q, isError=%v", text, isError)
	}
	if fake.publishedDocument != `[{"version":"v0.9"}]` {
		t.Fatalf("published document = %q", fake.publishedDocument)
	}
	if _, isError := callText(t, session, "surface", map[string]any{"action": "list", "session_id": "b2"}); isError || fake.surfacesOf != "b2" {
		t.Fatalf("list forwarded %q, isError=%v", fake.surfacesOf, isError)
	}
	if text, isError := callText(t, session, "surface", map[string]any{"action": "show", "surface_id": "s1", "width": 40}); isError ||
		text != "drawn" || fake.shownSurface != "s1" || fake.shownWidth != 40 {
		t.Fatalf("show = %q (%s at %d), isError=%v", text, fake.shownSurface, fake.shownWidth, isError)
	}
}

func TestSurfaceRefusesAnEmptyDocumentAndAnUnknownAction(t *testing.T) {
	fake := &fakeSessionCommands{}
	session := connectServer(t, serverWithFakes(t, fake))
	if text, isError := callText(t, session, "surface", map[string]any{"action": "publish"}); !isError || !strings.Contains(text, "document is empty") {
		t.Fatalf("empty publish = %q, isError=%v", text, isError)
	}
	if fake.publishedDocument != "" {
		t.Fatal("an empty document reached the command layer")
	}
	if text, isError := callText(t, session, "surface", map[string]any{"action": "answer"}); !isError || !strings.Contains(text, "publish, list, show") {
		t.Fatalf("unknown action = %q, isError=%v", text, isError)
	}
}
