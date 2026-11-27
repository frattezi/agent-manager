package ui

import (
	"strings"
	"testing"

	uibots "github.com/YoanWai/agent-manager/internal/ui/bots"
)

// The live lane binds a request by matching Runner[S] against each Binder.
// A request no binder matches still runs, but returns "binds no services",
// so a spawn would refuse with that instead of readying the Bot.
func TestPrepareRequestBindsServicesOnTheLiveLane(t *testing.T) {
	t.Parallel()
	m, _, _ := specializingModel(t)
	work := m.bindEffect(uibots.PrepareRequest{
		Profile:    testBotProfile(t.TempDir()),
		Dir:        t.TempDir(),
		Specialize: true,
		Roots:      []string{t.TempDir()},
	})
	_, err := work()
	if err != nil && strings.Contains(err.Error(), "binds no services") {
		t.Fatalf("PrepareRequest binds no services on the live lane: %v", err)
	}
}
