package bots

import "testing"

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
