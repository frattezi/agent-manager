package bots

import (
	"slices"
	"testing"
)

func TestFactsListWhatTheProfileSetsAndNameTheDefaultModel(t *testing.T) {
	got := Profile{Name: "operator", Emoji: "🛠", Tool: "claude", Dir: "/work", MemoryProject: "ops"}.Facts()
	want := [][2]string{{"Profile", "operator"}, {"Emoji", "🛠"}, {"Tool", "claude"}, {"Model", "the CLI's own default"}, {"Directory", "/work"}, {"Memory", "ops"}}
	if !slices.Equal(got, want) {
		t.Fatalf("facts = %v, want %v", got, want)
	}
}
