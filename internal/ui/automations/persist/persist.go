// Package persist is the store as the Automations commands and the clock
// read and write it, shared by the manager and agent-manager automations.
package persist

import (
	"errors"
	"fmt"
	"time"

	"github.com/YoanWai/agent-manager/internal/store"
	"github.com/YoanWai/agent-manager/internal/ui/automations"
)

// Book answers automations.Book from the store.
type Book struct{ Store *store.Store }

var _ automations.Book = Book{}

func (b Book) AutomationRows() ([]automations.Row, error) {
	stored, err := b.Store.Automations()
	if err != nil {
		return nil, err
	}
	rows := make([]automations.Row, len(stored))
	for i, a := range stored {
		rows[i] = Row(a)
	}
	return rows, nil
}

func (b Book) AutomationRow(id string) (automations.Row, bool, error) {
	a, ok, err := b.Store.Automation(id)
	return Row(a), ok, err
}

func (b Book) SetLastFired(id string, firedFor time.Time) error {
	return notFound(id, b.Store.SetAutomationLastFired(id, firedFor))
}

func (b Book) AddAutomation(row automations.Row, now time.Time) error {
	err := b.Store.AddAutomation(store.Automation{ID: row.ID, Name: row.Name, Config: row.Document, Enabled: row.Enabled, LastFiredFor: row.LastFiredFor}, now)
	if errors.Is(err, store.ErrAutomationTaken) {
		return automations.ErrTaken
	}
	return err
}

func (b Book) UpdateAutomation(id, name string, document []byte, now time.Time) error {
	return notFound(id, b.Store.UpdateAutomation(id, name, document, now))
}

func (b Book) SetAutomationEnabled(id string, enabled bool, now time.Time) error {
	return notFound(id, b.Store.SetAutomationEnabled(id, enabled, now))
}

func (b Book) DeleteAutomation(id string) error {
	return notFound(id, b.Store.DeleteAutomation(id))
}

// notFound lets a caller test for automations.ErrNotFound, keeping the
// store's sentence.
func notFound(id string, err error) error {
	if errors.Is(err, store.ErrAutomationNotFound) {
		return fmt.Errorf("automation %q: %w", id, automations.ErrNotFound)
	}
	return err
}

// Row is a stored Automation as the commands read it.
func Row(a store.Automation) automations.Row {
	return automations.Row{ID: a.ID, Name: a.Name, Document: a.Config, Enabled: a.Enabled, LastFiredFor: a.LastFiredFor, CreatedAt: a.CreatedAt}
}

// Services is the store and the Fractory directories the clock and run now
// work against.
func Services(st *store.Store) automations.Services {
	return automations.Services{Store: Book{st}, Book: Book{st}, RequestsDir: automations.RequestsDir(), RecordsDir: automations.RecordsDir()}
}
