package ui

import (
	"time"

	"github.com/YoanWai/agent-manager/internal/store"
	uibots "github.com/YoanWai/agent-manager/internal/ui/bots"
)

// botBook is the bot_profiles table as a Profile's save reads and writes it.
type botBook struct{ store *store.Store }

func (b botBook) BotProfile(id string) (uibots.Profile, bool, error) {
	p, ok, err := b.store.BotProfile(id)
	return uibots.Profile{
		ID: p.ID, Name: p.Name, Emoji: p.Emoji, Color: p.Color, Tool: p.Tool, Model: p.Model, Dir: p.Dir,
		Flags: p.Flags, Specialization: p.Specialization, MemoryProject: p.MemoryProject,
	}, ok, err
}

func (b botBook) AddBotProfile(p uibots.Profile, now time.Time) error {
	return b.store.AddBotProfile(storedBot(p), now)
}

func (b botBook) UpdateBotProfile(p uibots.Profile, now time.Time) error {
	return b.store.UpdateBotProfile(storedBot(p), now)
}

func storedBot(p uibots.Profile) store.BotProfile {
	return store.BotProfile{
		ID: p.ID, Name: p.Name, Emoji: p.Emoji, Color: p.Color, Tool: p.Tool, Model: p.Model, Dir: p.Dir,
		Flags: p.Flags, Specialization: p.Specialization, MemoryProject: p.MemoryProject,
	}
}

func (s effectServices) botServices() uibots.Services {
	return uibots.Services{Book: botBook{s.store}}
}
