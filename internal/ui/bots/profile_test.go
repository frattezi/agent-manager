package bots

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestSpecsNameEveryFieldOnce(t *testing.T) {
	specs := Specs([]string{"claude", "codex"}, 3)
	if len(specs) != len(Fields) {
		t.Fatalf("%d specs for %d fields", len(specs), len(Fields))
	}
	for i, spec := range specs {
		if spec.Key != Fields[i].Key {
			t.Fatalf("spec %d is %s, want %s", i, spec.Key, Fields[i].Key)
		}
		switch spec.Key {
		case ColorKey:
			if !slices.Equal(spec.Values, []string{"0", "1", "2"}) || spec.Default != "0" {
				t.Fatalf("color = %+v, want the three swatch indices", spec)
			}
		case ToolKey:
			if !slices.Equal(spec.Values, []string{"claude", "codex"}) {
				t.Fatalf("tool = %+v, want the agent CLIs", spec)
			}
		default:
			if spec.Values != nil {
				t.Fatalf("%s takes %v, want any text", spec.Key, spec.Values)
			}
		}
	}
}

func TestSetAndGetRoundTripEveryField(t *testing.T) {
	var p Profile
	for i, field := range Fields {
		value := "v" + field.Flag
		if field.Key == ColorKey {
			value = "2"
		}
		if err := p.Set(field.Key, value); err != nil {
			t.Fatalf("Set %s: %v", field.Key, err)
		}
		if got := p.Get(field.Key); got != value {
			t.Fatalf("field %d %s = %q, want %q", i, field.Key, got, value)
		}
	}
	if err := p.Set(ColorKey, "teal"); err == nil || !strings.Contains(err.Error(), ColorKey) {
		t.Fatalf("Set color teal = %v, want it refused against the field", err)
	}
	if err := p.Set("bot.voice", "x"); err == nil {
		t.Fatal("Set of an unknown field succeeded")
	}
}

func TestAdmitNamesTheFieldItRefuses(t *testing.T) {
	ok := Profile{Name: "operator", Tool: "claude"}
	if err := ok.Admit(); err != nil {
		t.Fatalf("Admit = %v", err)
	}
	for _, tt := range []struct {
		profile Profile
		field   string
	}{
		{Profile{Tool: "claude"}, NameKey},
		{Profile{Name: strings.Repeat("n", 61), Tool: "claude"}, NameKey},
		{Profile{Name: "operator"}, ToolKey},
		{Profile{Name: "operator", Tool: "claude", Flags: "--a\n--b"}, FlagsKey},
	} {
		if err := tt.profile.Admit(); err == nil || !strings.Contains(err.Error(), tt.field) {
			t.Errorf("Admit(%+v) = %v, want %s refused", tt.profile, err, tt.field)
		}
	}
}

func TestFindTakesAnIDOrAUniqueName(t *testing.T) {
	profiles := []Profile{{ID: "a", Name: "ops"}, {ID: "b", Name: "watch"}, {ID: "c", Name: "watch"}}
	if p, err := Find(profiles, "a"); err != nil || p.ID != "a" {
		t.Fatalf("by id = %+v, %v", p, err)
	}
	if p, err := Find(profiles, "ops"); err != nil || p.ID != "a" {
		t.Fatalf("by name = %+v, %v", p, err)
	}
	if _, err := Find(profiles, "watch"); !errors.Is(err, ErrAmbiguous) || !strings.Contains(err.Error(), "b, c") {
		t.Fatalf("shared name = %v, want both ids named", err)
	}
	if _, err := Find(profiles, "nosuch"); err == nil {
		t.Fatal("missing name found something")
	}
}
