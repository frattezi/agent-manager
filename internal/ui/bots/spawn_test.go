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

func TestFirstPromptNamesTheMemoryAndNoSkill(t *testing.T) {
	prompt := FirstPrompt(Profile{Name: "Operator", Specialization: "fractory-operator", MemoryProject: "fractory-ops"})
	for _, want := range []string{`"Operator"`, `ai-memory project "fractory-ops"`} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "skill") || strings.Contains(prompt, "fractory-operator") {
		t.Errorf("the first prompt still loads the Specialization:\n%s", prompt)
	}
	if empty := FirstPrompt(Profile{Name: "Operator", Specialization: "fractory-operator"}); empty != "" {
		t.Errorf("a Profile with no memory still sends %q", empty)
	}
}
