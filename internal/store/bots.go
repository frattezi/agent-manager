package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrBotProfileTaken reports an id another Bot Profile already holds.
var ErrBotProfileTaken = errors.New("bot profile id is taken")

// ErrBotProfileNotFound reports an id no Bot Profile carries.
var ErrBotProfileNotFound = errors.New("bot profile does not exist")

// BotProfile is one stored Bot Profile. internal/ui/bots validates its
// fields; the store keeps them as written.
type BotProfile struct {
	ID             string
	Name           string
	Emoji          string
	Color          int
	Tool           string
	Model          string
	Dir            string
	Flags          string
	Specialization string
	MemoryProject  string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

const botProfileColumns = `id, name, emoji, color, tool, model, dir, flags, specialization, memory_project, created_at, updated_at`

func scanBotProfile(row interface{ Scan(...any) error }) (BotProfile, error) {
	var p BotProfile
	var created, updated int64
	if err := row.Scan(&p.ID, &p.Name, &p.Emoji, &p.Color, &p.Tool, &p.Model, &p.Dir, &p.Flags, &p.Specialization, &p.MemoryProject, &created, &updated); err != nil {
		return BotProfile{}, err
	}
	p.CreatedAt = decodeTime(created)
	p.UpdatedAt = decodeTime(updated)
	return p, nil
}

// BotProfiles lists every Bot Profile in the order they were added.
func (s *Store) BotProfiles() ([]BotProfile, error) {
	rows, err := s.db.Query(`SELECT ` + botProfileColumns + ` FROM bot_profiles ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	profiles := []BotProfile{}
	for rows.Next() {
		p, err := scanBotProfile(rows)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	return profiles, rows.Err()
}

func (s *Store) BotProfile(id string) (BotProfile, bool, error) {
	p, err := scanBotProfile(s.db.QueryRow(`SELECT `+botProfileColumns+` FROM bot_profiles WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return BotProfile{}, false, nil
	}
	if err != nil {
		return BotProfile{}, false, err
	}
	return p, true, nil
}

// AddBotProfile stores a new Bot Profile, refusing an id another one
// holds. Its created and updated stamps are now.
func (s *Store) AddBotProfile(p BotProfile, now time.Time) error {
	res, err := s.db.Exec(
		`INSERT INTO bot_profiles (`+botProfileColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO NOTHING`,
		p.ID, p.Name, p.Emoji, p.Color, p.Tool, p.Model, p.Dir, p.Flags, p.Specialization, p.MemoryProject, encodeTime(now), encodeTime(now))
	if err != nil {
		return err
	}
	return affectedOne(res, fmt.Errorf("bot profile %q: %w", p.ID, ErrBotProfileTaken))
}

// UpdateBotProfile replaces every field of the Bot Profile p.ID but its
// created stamp.
func (s *Store) UpdateBotProfile(p BotProfile, now time.Time) error {
	res, err := s.db.Exec(
		`UPDATE bot_profiles SET name = ?, emoji = ?, color = ?, tool = ?, model = ?, dir = ?, flags = ?,
		 specialization = ?, memory_project = ?, updated_at = ? WHERE id = ?`,
		p.Name, p.Emoji, p.Color, p.Tool, p.Model, p.Dir, p.Flags, p.Specialization, p.MemoryProject, encodeTime(now), p.ID)
	if err != nil {
		return err
	}
	return affectedOne(res, fmt.Errorf("bot profile %q: %w", p.ID, ErrBotProfileNotFound))
}
