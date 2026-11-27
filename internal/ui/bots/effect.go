package bots

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Book is the store as a Profile's save reads and writes it, the same
// bot_profiles table agent-manager bots opens.
type Book interface {
	BotProfile(id string) (Profile, bool, error)
	AddBotProfile(p Profile, now time.Time) error
	UpdateBotProfile(p Profile, now time.Time) error
}

// Services is what a Profile's save runs against on the effect lane.
type Services struct{ Book Book }

// SaveRequest stores the dialog's Profile: a new one under a new id when
// Editing is empty, else a replacement for the one Editing names, whose id
// never changes. Gen is the dialog that asked. Tools and Swatches are the
// values the tool and colour take, read when the save was asked.
type SaveRequest struct {
	Gen      uint64
	Editing  string
	Profile  Profile
	Tools    []string
	Swatches int
	Now      time.Time
}

func (SaveRequest) EffectName() string { return "Bot Profile save" }

// Run refuses a field the shell would refuse, as a FieldError, and reads
// the stored Profile back.
func (r SaveRequest) Run(services Services) (any, error) {
	p := r.Profile
	p.ID = r.Editing
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if err := p.AdmitAmong(r.Tools, r.Swatches); err != nil {
		return Profile{}, err
	}
	var err error
	if r.Editing == "" {
		err = services.Book.AddBotProfile(p, r.Now)
	} else {
		err = services.Book.UpdateBotProfile(p, r.Now)
	}
	if err != nil {
		return Profile{}, err
	}
	stored, ok, err := services.Book.BotProfile(p.ID)
	if err != nil {
		return Profile{}, err
	}
	if !ok {
		return Profile{}, fmt.Errorf("bot profile %q was saved and is gone", p.ID)
	}
	return stored, nil
}
