package a2ui

import (
	"errors"
	"strings"
	"testing"
)

func TestParseRefusesAnotherVersionNamingBoth(t *testing.T) {
	for _, tc := range []struct {
		name, doc, found, want string
	}{
		{"older", strings.Replace(surface(`{"id":"root","component":"Divider"}`), `"v0.9"`, `"v0.8"`, 1), "v0.8",
			`messages[0].version is "v0.8"; this build reads A2UI "v0.9"`},
		{"later message", strings.Replace(surface(`{"id":"root","component":"Divider"}`), `{"version":"v0.9","updateComponents"`, `{"version":"v1.0","updateComponents"`, 1), "v1.0",
			`messages[1].version is "v1.0"; this build reads A2UI "v0.9"`},
		{"missing", `[{"createSurface":{"surfaceId":"s"}}]`, "",
			`messages[0].version is missing; this build reads A2UI "v0.9"`},
		{"not a string", `[{"version":0.9,"createSurface":{"surfaceId":"s"}}]`, "0.9",
			`messages[0].version is "0.9"; this build reads A2UI "v0.9"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.doc))
			var version *VersionError
			if !errors.As(err, &version) {
				t.Fatalf("err = %v, want a VersionError", err)
			}
			if version.Found != tc.found || version.Expected != Version || err.Error() != tc.want {
				t.Fatalf("err = %q (found %q), want %q", err, version.Found, tc.want)
			}
		})
	}
}

func TestParseRefusesAMalformedDocumentNamingTheField(t *testing.T) {
	for _, tc := range []struct {
		name, doc, field, problem string
	}{
		{"not json", `[{`, "document", "must be a JSON array"},
		{"not an array", `{"version":"v0.9"}`, "document", "must be a JSON array"},
		{"empty", `[]`, "document", "holds no messages"},
		{"message not an object", `["x"]`, "messages[0]", "must be an object"},
		{"no components", `[{"version":"v0.9","createSurface":{"surfaceId":"s"}}]`, "document", "has no updateComponents"},
		{"no surface id", `[{"version":"v0.9","updateComponents":{"components":[]}}]`, "messages[0].updateComponents.surfaceId", "is required"},
		{"two surfaces", `[{"version":"v0.9","createSurface":{"surfaceId":"a"}},{"version":"v0.9","updateComponents":{"surfaceId":"b","components":[]}}]`,
			"messages[1].updateComponents.surfaceId", `is "b", but this document describes surface "a"`},
		{"components not a list", `[{"version":"v0.9","updateComponents":{"surfaceId":"s","components":{}}}]`, "messages[0].updateComponents.components", "must be an array"},
		{"no root", surface(`{"id":"top","component":"Divider"}`), "document", `no component has id "root"`},
		{"no id", surface(`{"component":"Divider"}`), "messages[1].updateComponents.components[0].id", "is required"},
		{"no type", surface(`{"id":"root"}`), "messages[1].updateComponents.components[0].component", "is required"},
		{"text missing", surface(`{"id":"root","component":"Text"}`), "messages[1].updateComponents.components[0].text", "is required"},
		{"text a number", surface(`{"id":"root","component":"Text","text":3}`), "messages[1].updateComponents.components[0].text", "must be a string"},
		{"binding without path", surface(`{"id":"root","component":"Text","text":{"call":"now"}}`), "messages[1].updateComponents.components[0].text", `{"path": "/pointer"} binding`},
		{"relative binding", surface(`{"id":"root","component":"Text","text":{"path":"name"}}`), "messages[1].updateComponents.components[0].text.path", `must start with "/"`},
		{"missing child", surface(`{"id":"root","component":"Column","children":["gone"]}`), "messages[1].updateComponents.components[0].children[0]", `no component has id "gone"`},
		{"cycle", surface(`{"id":"root","component":"Card","child":"inner"},{"id":"inner","component":"Column","children":["root"]}`),
			"messages[1].updateComponents.components[1].children[0]", `"root" contains itself`},
		{"card without child", surface(`{"id":"root","component":"Card"}`), "messages[1].updateComponents.components[0].child", "is required"},
		{"children not ids", surface(`{"id":"root","component":"Row","children":[1]}`), "messages[1].updateComponents.components[0].children", "must be an array of component ids"},
		{"no options", surface(`{"id":"root","component":"ChoicePicker","options":[]}`), "messages[1].updateComponents.components[0].options", "at least one choice"},
		{"option without label", surface(`{"id":"root","component":"ChoicePicker","options":[{"value":"a"}]}`), "messages[1].updateComponents.components[0].options[0].label", "is required"},
		{"option without value", surface(`{"id":"root","component":"ChoicePicker","options":[{"label":"A"}]}`), "messages[1].updateComponents.components[0].options[0].value", "is required"},
		{"choice not offered", surface(`{"id":"root","component":"ChoicePicker","options":[{"label":"A","value":"a"}],"value":["b"]}`),
			"messages[1].updateComponents.components[0].value[0]", `"b" is not the value of any of the options`},
		{"two choices of one", surface(`{"id":"root","component":"ChoicePicker","options":[{"label":"A","value":"a"},{"label":"B","value":"b"}],"value":["a","b"]}`),
			"messages[1].updateComponents.components[0].value", "chooses more than one option"},
		{"check box not a flag", surface(`{"id":"root","component":"CheckBox","label":"x","value":"yes"}`), "messages[1].updateComponents.components[0].value", "must be true or false"},
		{"field without label", surface(`{"id":"root","component":"TextField"}`), "messages[1].updateComponents.components[0].label", "is required"},
		{"data path relative", `[{"version":"v0.9","updateDataModel":{"surfaceId":"s","path":"a","value":1}}]`, "messages[0].updateDataModel.path", `must start with "/"`},
		{"data under a scalar", `[{"version":"v0.9","updateDataModel":{"surfaceId":"s","path":"/a","value":1}},{"version":"v0.9","updateDataModel":{"surfaceId":"s","path":"/a/b","value":2}}]`,
			"messages[1].updateDataModel.path", "neither an object nor an array"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.doc))
			var refusal *Error
			if !errors.As(err, &refusal) {
				t.Fatalf("err = %v, want an Error", err)
			}
			if refusal.Field != tc.field || !strings.Contains(refusal.Problem, tc.problem) {
				t.Fatalf("err = %q, want field %q with %q", err, tc.field, tc.problem)
			}
		})
	}
}

func TestParseRefusesATreeThatExplodes(t *testing.T) {
	var components []string
	for i := range 20 {
		next := "leaf"
		if i < 19 {
			next = "n" + string(rune('a'+i+1))
		}
		id := "n" + string(rune('a'+i))
		if i == 0 {
			id = "root"
		}
		components = append(components, `{"id":"`+id+`","component":"Column","children":["`+next+`","`+next+`"]}`)
	}
	components = append(components, `{"id":"leaf","component":"Divider"}`)
	_, err := Parse([]byte(surface(strings.Join(components, ","))))
	var refusal *Error
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Problem, "expands past") {
		t.Fatalf("err = %v, want the tree refused", err)
	}
}

func TestParseIgnoresUnknownFieldsAndMessages(t *testing.T) {
	doc := fixture(t, "unknown")
	if doc.SurfaceID != "newer" || len(doc.Root.Children) != 3 {
		t.Fatalf("doc = %+v", doc)
	}
	gap := doc.Root.Children[1]
	if !gap.Gap || gap.Type != "Slider" || gap.ID != "volume" || gap.Reason == "" {
		t.Fatalf("slider = %+v, want a labelled gap", gap)
	}
}

func TestParseResolvesBindingsAgainstTheDataModel(t *testing.T) {
	doc := fixture(t, "round")
	q1 := doc.Root.Children[0].Children[1]
	if q1.Type != "ChoicePicker" || !q1.Options[0].Selected || q1.Options[1].Selected {
		t.Fatalf("q1 = %+v, want the bound recommendation chosen", q1)
	}
	other := doc.Root.Children[0].Children[4]
	if other.Type != "TextField" || other.Value != "" {
		t.Fatalf("other = %+v, want an empty field for a path the model lacks", other)
	}
}

func TestParseAppliesDataModelUpdatesInOrder(t *testing.T) {
	doc := `[{"version":"v0.9","updateComponents":{"surfaceId":"s","components":[{"id":"root","component":"Text","text":{"path":"/a~1b/0"}}]}},
		{"version":"v0.9","updateDataModel":{"surfaceId":"s","value":{"a/b":["first"],"gone":1}}},
		{"version":"v0.9","updateDataModel":{"surfaceId":"s","path":"/a~1b/0","value":"second"}},
		{"version":"v0.9","updateDataModel":{"surfaceId":"s","path":"/gone"}}]`
	parsed, err := Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Root.Text != "second" {
		t.Fatalf("text = %q, want the later update", parsed.Root.Text)
	}
}

func TestParseLetsALaterUpdateReplaceAComponent(t *testing.T) {
	doc := `[{"version":"v0.9","updateComponents":{"surfaceId":"s","components":[{"id":"root","component":"Text","text":"draft"}]}},
		{"version":"v0.9","updateComponents":{"surfaceId":"s","components":[{"id":"root","component":"Text","text":"final"}]}}]`
	parsed, err := Parse([]byte(doc))
	if err != nil || parsed.Root.Text != "final" {
		t.Fatalf("parse = %+v, %v", parsed.Root, err)
	}
}
