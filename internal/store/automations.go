package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrAutomationTaken reports an id another Automation already holds.
var ErrAutomationTaken = errors.New("automation id is taken")

// ErrAutomationNotFound reports an id no Automation carries.
var ErrAutomationNotFound = errors.New("automation does not exist")

// Automation is one stored Automation. internal/ui/automations validates
// Config; the store keeps its bytes as written.
type Automation struct {
	ID           string
	Name         string
	Config       []byte
	Enabled      bool
	LastFiredFor time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

const automationColumns = `id, name, config, enabled, last_fired_for, created_at, updated_at`

func scanAutomation(row interface{ Scan(...any) error }) (Automation, error) {
	var a Automation
	var config string
	var enabled int
	var lastFired, created, updated int64
	if err := row.Scan(&a.ID, &a.Name, &config, &enabled, &lastFired, &created, &updated); err != nil {
		return Automation{}, err
	}
	a.Config = []byte(config)
	a.Enabled = enabled == 1
	a.LastFiredFor = decodeTime(lastFired)
	a.CreatedAt = decodeTime(created)
	a.UpdatedAt = decodeTime(updated)
	return a, nil
}

// Automations lists every Automation in the order they were added.
func (s *Store) Automations() ([]Automation, error) {
	rows, err := s.db.Query(`SELECT ` + automationColumns + ` FROM automations ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	automations := []Automation{}
	for rows.Next() {
		a, err := scanAutomation(rows)
		if err != nil {
			return nil, err
		}
		automations = append(automations, a)
	}
	return automations, rows.Err()
}

func (s *Store) Automation(id string) (Automation, bool, error) {
	a, err := scanAutomation(s.db.QueryRow(`SELECT `+automationColumns+` FROM automations WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Automation{}, false, nil
	}
	if err != nil {
		return Automation{}, false, err
	}
	return a, true, nil
}

// AddAutomation stores a new Automation, refusing an id another one holds.
// Its created and updated stamps are now; LastFiredFor is kept as given.
func (s *Store) AddAutomation(a Automation, now time.Time) error {
	res, err := s.db.Exec(
		`INSERT INTO automations (`+automationColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO NOTHING`,
		a.ID, a.Name, string(a.Config), boolToInt(a.Enabled), encodeTime(a.LastFiredFor), encodeTime(now), encodeTime(now))
	if err != nil {
		return err
	}
	return affectedOne(res, fmt.Errorf("automation %q: %w", a.ID, ErrAutomationTaken))
}

// UpdateAutomation replaces an Automation's name and configuration. Its id,
// enabled flag and watermark stay as they are.
func (s *Store) UpdateAutomation(id, name string, config []byte, now time.Time) error {
	res, err := s.db.Exec(`UPDATE automations SET name = ?, config = ?, updated_at = ? WHERE id = ?`,
		name, string(config), encodeTime(now), id)
	if err != nil {
		return err
	}
	return affectedOne(res, automationNotFound(id))
}

func (s *Store) SetAutomationEnabled(id string, enabled bool, now time.Time) error {
	res, err := s.db.Exec(`UPDATE automations SET enabled = ?, updated_at = ? WHERE id = ?`,
		boolToInt(enabled), encodeTime(now), id)
	if err != nil {
		return err
	}
	return affectedOne(res, automationNotFound(id))
}

// SetAutomationLastFired advances the watermark: the instant the Automation
// last fired for. It is state, not a change the owner made, so the updated
// stamp stays.
func (s *Store) SetAutomationLastFired(id string, firedFor time.Time) error {
	res, err := s.db.Exec(`UPDATE automations SET last_fired_for = ? WHERE id = ?`, encodeTime(firedFor), id)
	if err != nil {
		return err
	}
	return affectedOne(res, automationNotFound(id))
}

func (s *Store) DeleteAutomation(id string) error {
	res, err := s.db.Exec(`DELETE FROM automations WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return affectedOne(res, automationNotFound(id))
}

func automationNotFound(id string) error {
	return fmt.Errorf("automation %q: %w", id, ErrAutomationNotFound)
}

func affectedOne(res sql.Result, none error) error {
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return none
	}
	return nil
}
