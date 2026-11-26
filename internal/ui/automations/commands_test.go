package automations

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeBook is fakeStore with the writes the commands make.
type fakeBook struct{ *fakeStore }

func (b fakeBook) AddAutomation(row Row, now time.Time) error {
	if _, taken := b.rows[row.ID]; taken {
		return ErrTaken
	}
	row.CreatedAt = now
	b.rows[row.ID] = row
	return nil
}

func (b fakeBook) UpdateAutomation(id, name string, document []byte, _ time.Time) error {
	row, ok := b.rows[id]
	if !ok {
		return ErrNotFound
	}
	row.Name, row.Document = name, document
	b.rows[id] = row
	return nil
}

func (b fakeBook) SetAutomationEnabled(id string, enabled bool, _ time.Time) error {
	row, ok := b.rows[id]
	if !ok {
		return ErrNotFound
	}
	row.Enabled = enabled
	b.rows[id] = row
	return nil
}

func (b fakeBook) DeleteAutomation(id string) error {
	if _, ok := b.rows[id]; !ok {
		return ErrNotFound
	}
	delete(b.rows, id)
	return nil
}

func fractoryRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".fractory.json"), []byte(`{"mode":"work"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

func commandConfig(t *testing.T) Config {
	return Config{Version: Version, Name: "weekly", Kind: KindScript, RepoPath: fractoryRepo(t), Cron: "0 7 * * 1",
		Timezone: "UTC", Mode: ModeWork, Flags: []string{}, Entry: "scripts/weekly.sh", Retention: 3}
}

func fieldOf(err error) string {
	var field *FieldError
	if errors.As(err, &field) {
		return field.Field
	}
	return ""
}

func TestAddAssignsAnIDAndStoresTheConfiguration(t *testing.T) {
	book := fakeBook{&fakeStore{rows: map[string]Row{}}}
	stored, err := Add(book, commandConfig(t), true, nil, day(1, 6, 0))
	if err != nil {
		t.Fatal(err)
	}
	row, ok := book.rows[stored.ID]
	if stored.ID == "" || !ok || !row.Enabled || row.Name != "weekly" {
		t.Fatalf("stored %+v as %+v", stored, row)
	}
	var back Config
	if err := json.Unmarshal(row.Document, &back); err != nil || back.ID != stored.ID || back.Entry != "scripts/weekly.sh" {
		t.Fatalf("document %s: %v", row.Document, err)
	}
}

func TestAddRefusesWhatCouldNeverRunAndStoresNothing(t *testing.T) {
	for name, tc := range map[string]struct {
		edit  func(*Config)
		field string
	}{
		"malformed cron":            {func(c *Config) { c.Cron = "every monday" }, "cron"},
		"missing entry":             {func(c *Config) { c.Entry = " " }, "entry"},
		"not a Fractory repository": {func(c *Config) { c.RepoPath = t.TempDir() }, "repoPath"},
		"missing directory":         {func(c *Config) { c.RepoPath = filepath.Join(t.TempDir(), "gone") }, "repoPath"},
	} {
		t.Run(name, func(t *testing.T) {
			book := fakeBook{&fakeStore{rows: map[string]Row{}}}
			config := commandConfig(t)
			tc.edit(&config)
			_, err := Add(book, config, true, nil, day(1, 6, 0))
			if fieldOf(err) != tc.field {
				t.Fatalf("Add = %v; want a refusal naming %s", err, tc.field)
			}
			if len(book.rows) != 0 {
				t.Fatalf("a refused Automation was stored: %+v", book.rows)
			}
		})
	}
}

func TestChangeKeepsTheIDAndRefusesAnother(t *testing.T) {
	book := fakeBook{&fakeStore{rows: map[string]Row{}}}
	stored, err := Add(book, commandConfig(t), false, nil, day(1, 6, 0))
	if err != nil {
		t.Fatal(err)
	}
	next := commandConfig(t)
	next.Name, next.ID = "monday sweep", ""
	changed, err := Change(book, stored.ID, next, nil, day(2, 6, 0))
	if err != nil || changed.ID != stored.ID || book.rows[stored.ID].Name != "monday sweep" || book.rows[stored.ID].Enabled {
		t.Fatalf("Change = %+v, %v; row %+v", changed, err, book.rows[stored.ID])
	}
	next.ID = "other"
	if _, err := Change(book, stored.ID, next, nil, day(2, 6, 0)); fieldOf(err) != "id" {
		t.Fatalf("Change to another id = %v", err)
	}
}

func TestRunNowRequestsTheCurrentInstantAndLeavesTheWatermark(t *testing.T) {
	watermark := day(1, 7, 0)
	store, services := newFixture(t, Row{Enabled: true, LastFiredFor: watermark})
	now := day(1, 9, 30).Add(1500 * time.Millisecond)
	result, err := RunNow(services, "daily", now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Skipped || !result.FiredFor.Equal(day(1, 9, 30).Add(time.Second)) {
		t.Fatalf("result = %+v", result)
	}
	if got := requestFiles(t, services.RequestsDir); len(got) != 1 || got[0] != "daily@20261101T093001Z.json" {
		t.Fatalf("requests = %v", got)
	}
	if got := store.rows["daily"].LastFiredFor; !got.Equal(watermark) || store.advances != 0 {
		t.Fatalf("watermark = %v after %d advances; want it left at %v", got, store.advances, watermark)
	}
}

func TestRunNowIsSkippedWhileAFiringIsUnfinished(t *testing.T) {
	_, services := newFixture(t, Row{Enabled: true})
	writeRecord(t, services.RecordsDir, StatusRunning, day(1, 7, 0))
	result, err := RunNow(services, "daily", day(1, 9, 0))
	if err != nil || !result.Skipped {
		t.Fatalf("RunNow = %+v, %v; want it skipped", result, err)
	}
	if got := requestFiles(t, services.RequestsDir); len(got) != 0 {
		t.Fatalf("a skipped run now wrote %v", got)
	}

	_, services = newFixture(t, Row{Enabled: true})
	if _, err := RunNow(services, "daily", day(1, 9, 0)); err != nil {
		t.Fatal(err)
	}
	result, err = RunNow(services, "daily", day(1, 9, 5))
	if err != nil || !result.Skipped {
		t.Fatalf("a second run now while the first is queued = %+v, %v", result, err)
	}
}

func TestRunNowRefusesAnUnknownAutomation(t *testing.T) {
	_, services := newFixture(t, Row{Enabled: true})
	if _, err := RunNow(services, "gone", day(1, 9, 0)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RunNow = %v", err)
	}
}
