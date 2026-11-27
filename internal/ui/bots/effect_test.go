package bots

import (
	"errors"
	"testing"
	"time"
)

type memoryBook map[string]Profile

func (b memoryBook) BotProfile(id string) (Profile, bool, error) {
	p, ok := b[id]
	return p, ok, nil
}

func (b memoryBook) AddBotProfile(p Profile, _ time.Time) error {
	if _, ok := b[p.ID]; ok {
		return errors.New("taken")
	}
	b[p.ID] = p
	return nil
}

func (b memoryBook) UpdateBotProfile(p Profile, _ time.Time) error {
	if _, ok := b[p.ID]; !ok {
		return errors.New("missing")
	}
	b[p.ID] = p
	return nil
}

func TestSaveAddsUnderANewIDAndEditsInPlace(t *testing.T) {
	book := memoryBook{}
	services := Services{Book: book}
	request := SaveRequest{Profile: Profile{Name: "operator", Tool: "claude"}, Tools: []string{"claude"}, Swatches: 3}
	got, err := request.Run(services)
	added, _ := got.(Profile)
	if err != nil || added.ID == "" || book[added.ID].Name != "operator" {
		t.Fatalf("add = %+v, %v", got, err)
	}
	edit := SaveRequest{Editing: added.ID, Profile: Profile{ID: "other", Name: "ops", Tool: "claude", Color: 2}, Tools: []string{"claude"}, Swatches: 3}
	got, err = edit.Run(services)
	if saved, _ := got.(Profile); err != nil || saved.ID != added.ID || saved.Name != "ops" || len(book) != 1 {
		t.Fatalf("edit = %+v, %v; book %+v", got, err, book)
	}
}

func TestSaveRefusesWhatTheShellRefusesAgainstTheField(t *testing.T) {
	for _, tt := range []struct {
		profile Profile
		key     string
	}{
		{Profile{Tool: "claude"}, NameKey},
		{Profile{Name: "ops", Tool: "gone"}, ToolKey},
		{Profile{Name: "ops", Tool: "claude", Color: 3}, ColorKey},
		{Profile{Name: "ops", Tool: "claude", MemoryProject: "a\tb"}, MemoryKey},
	} {
		book := memoryBook{}
		_, err := SaveRequest{Profile: tt.profile, Tools: []string{"claude"}, Swatches: 3}.Run(Services{Book: book})
		var refused *FieldError
		if !errors.As(err, &refused) || refused.Key != tt.key || len(book) != 0 {
			t.Errorf("%+v: err %v, want %s refused and nothing stored", tt.profile, err, tt.key)
		}
	}
}
