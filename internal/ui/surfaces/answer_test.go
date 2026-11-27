package surfaces

import (
	"strings"
	"testing"

	"github.com/YoanWai/agent-manager/internal/ui/a2ui"
)

func parse(t *testing.T, document string) a2ui.Document {
	t.Helper()
	doc, err := a2ui.Parse([]byte(document))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestTheAnswerNamesEachQuestionAndWhetherItKeptTheRecommendation(t *testing.T) {
	round := document(`{"id":"root","component":"Column","children":["q","box","note"]},` +
		`{"id":"q","component":"ChoicePicker","label":"Pick","variant":"multipleSelection","options":[{"label":"A","value":"a"},{"label":"B","value":"b"}],"value":["a"]},` +
		`{"id":"box","component":"CheckBox","label":"Ship it","value":true},` +
		`{"id":"note","component":"TextField","label":"Else"}`)
	published, answered := parse(t, round), parse(t, round)
	fields := a2ui.Fields(&answered)
	fields[0].Options[1].Selected = true
	got := AnswerText(published, answered)
	want := strings.Join([]string{
		"Pick",
		"  A [a]; B [b] (overriding the recommended: A [a])",
		"Ship it",
		"  checked (the recommended answer)",
		"Else",
		"  (left empty)",
	}, "\n")
	if got != want {
		t.Fatalf("answer =\n%s\nwant\n%s", got, want)
	}
}

func TestARoundWithNothingToAnswerStillSendsAnAnswer(t *testing.T) {
	doc := parse(t, document(`{"id":"root","component":"Text","text":"Read this"}`))
	if got := AnswerText(doc, doc); got == "" {
		t.Fatal("an empty answer")
	}
}
