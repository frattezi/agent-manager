package bots

import (
	"strings"
	"testing"
)

func TestSpawnNamesEachBotFromAProfileApart(t *testing.T) {
	p := Profile{ID: "b1", Name: "Fractory Operator"}
	first := Spawn(p, []string{"other"})
	if first.Name != "fractory-operator-1" {
		t.Fatalf("first Bot = %q", first.Name)
	}
	second := Spawn(p, []string{"other", first.Name})
	if second.Name != "fractory-operator-2" {
		t.Fatalf("second Bot = %q", second.Name)
	}
	if again := Spawn(p, []string{second.Name}); again.Name != first.Name {
		t.Fatalf("a closed Bot's number is free again, got %q", again.Name)
	}
	if named := Spawn(Profile{ID: "b2", Name: "🤖"}, nil); named.Name != "b2-1" {
		t.Fatalf("a name with nothing to slug falls back to the id, got %q", named.Name)
	}
}

func TestFirstPromptLoadsTheSpecializationAndNamesTheMemory(t *testing.T) {
	prompt := FirstPrompt(Profile{Name: "Operator", Specialization: "fractory-operator", MemoryProject: "fractory-ops"})
	for _, want := range []string{`"Operator"`, "fractory-operator skill", `ai-memory project "fractory-ops"`} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q:\n%s", want, prompt)
		}
	}
	if only := FirstPrompt(Profile{Name: "Operator", MemoryProject: "m"}); strings.Contains(only, "skill") {
		t.Errorf("a Profile with no Specialization names a skill:\n%s", only)
	}
	if empty := FirstPrompt(Profile{Name: "Operator"}); empty != "" {
		t.Errorf("a Profile with nothing to load still sends %q", empty)
	}
}
