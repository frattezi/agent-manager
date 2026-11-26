package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func openAutomationStore(t *testing.T, path string) *Store {
	t.Helper()
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestAutomationStoredReadBackChangedAndRemoved(t *testing.T) {
	s := openAutomationStore(t, filepath.Join(t.TempDir(), "state.db"))
	if listed, err := s.Automations(); err != nil || listed == nil || len(listed) != 0 {
		t.Fatalf("empty Automations = %#v, %v; want an empty list", listed, err)
	}
	created := time.Date(2026, 11, 26, 9, 0, 0, 123, time.UTC)
	want := Automation{ID: "a1", Name: "weekly", Config: []byte(`{"id":"a1","name":"weekly"}`), Enabled: true}
	if err := s.AddAutomation(want, created); err != nil {
		t.Fatal(err)
	}
	got, found, err := s.Automation("a1")
	if err != nil || !found {
		t.Fatalf("Automation = %v, %v", found, err)
	}
	want.CreatedAt, want.UpdatedAt = created, created
	if !got.CreatedAt.Equal(created) || !got.UpdatedAt.Equal(created) {
		t.Fatalf("stamps = %v %v, want %v", got.CreatedAt, got.UpdatedAt, created)
	}
	got.CreatedAt, got.UpdatedAt = created, created
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("read back %+v, want %+v", got, want)
	}

	changed := created.Add(time.Hour)
	if err := s.UpdateAutomation("a1", "nightly", []byte(`{"id":"a1","name":"nightly"}`), changed); err != nil {
		t.Fatal(err)
	}
	got, _, _ = s.Automation("a1")
	if got.ID != "a1" || got.Name != "nightly" || string(got.Config) != `{"id":"a1","name":"nightly"}` {
		t.Fatalf("after rename = %+v, want id a1 kept", got)
	}
	if !got.CreatedAt.Equal(created) || !got.UpdatedAt.Equal(changed) {
		t.Fatalf("stamps after change = %v %v", got.CreatedAt, got.UpdatedAt)
	}

	if err := s.DeleteAutomation("a1"); err != nil {
		t.Fatal(err)
	}
	if _, found, err := s.Automation("a1"); found || err != nil {
		t.Fatalf("after delete found=%v err=%v", found, err)
	}
	for name, err := range map[string]error{
		"update": s.UpdateAutomation("a1", "x", []byte(`{}`), changed),
		"enable": s.SetAutomationEnabled("a1", true, changed),
		"fired":  s.SetAutomationLastFired("a1", changed),
		"delete": s.DeleteAutomation("a1"),
	} {
		if !errors.Is(err, ErrAutomationNotFound) {
			t.Errorf("%s on a removed Automation = %v, want ErrAutomationNotFound", name, err)
		}
	}
}

func TestAutomationIDsAreUnique(t *testing.T) {
	s := openAutomationStore(t, filepath.Join(t.TempDir(), "state.db"))
	now := time.Now()
	if err := s.AddAutomation(Automation{ID: "same", Name: "one", Config: []byte(`{}`)}, now); err != nil {
		t.Fatal(err)
	}
	err := s.AddAutomation(Automation{ID: "same", Name: "two", Config: []byte(`{}`)}, now)
	if !errors.Is(err, ErrAutomationTaken) {
		t.Fatalf("second add = %v, want ErrAutomationTaken", err)
	}
	got, _, _ := s.Automation("same")
	if got.Name != "one" {
		t.Fatalf("first Automation was overwritten: %+v", got)
	}
}

func TestAutomationStateIsIndependentOfConfiguration(t *testing.T) {
	s := openAutomationStore(t, filepath.Join(t.TempDir(), "state.db"))
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	config := []byte(`{"id":"a"}`)
	if err := s.AddAutomation(Automation{ID: "a", Name: "a", Config: config, Enabled: true}, created); err != nil {
		t.Fatal(err)
	}
	fired := time.Date(2026, 1, 5, 7, 0, 0, 0, time.UTC)
	if err := s.SetAutomationLastFired("a", fired); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAutomationEnabled("a", false, created.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, _, _ := s.Automation("a")
	if got.Enabled || !got.LastFiredFor.Equal(fired) || string(got.Config) != string(config) {
		t.Fatalf("after state changes = %+v", got)
	}
	if err := s.UpdateAutomation("a", "renamed", []byte(`{"id":"a","name":"renamed"}`), created.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, _, _ = s.Automation("a")
	if got.Enabled || !got.LastFiredFor.Equal(fired) {
		t.Fatalf("a configuration change touched the state: %+v", got)
	}
	if err := s.SetAutomationEnabled("a", true, created.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, _, _ = s.Automation("a")
	if !got.Enabled || !got.LastFiredFor.Equal(fired) {
		t.Fatalf("after enable = %+v", got)
	}
}

func automationColumnsOf(t *testing.T, s *Store) []string {
	t.Helper()
	rows, err := s.db.Query(`SELECT name, type, "notnull", COALESCE(dflt_value, ''), pk FROM pragma_table_info('automations') ORDER BY cid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var name, typ, dflt string
		var notNull, pk int
		if err := rows.Scan(&name, &typ, &notNull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, name+" "+typ+" "+dflt+" "+string(rune('0'+notNull))+string(rune('0'+pk)))
	}
	return columns
}

func TestAutomationsTableIsTheSameOnFreshAndUpgradedStores(t *testing.T) {
	fresh := automationColumnsOf(t, openAutomationStore(t, filepath.Join(t.TempDir(), "fresh.db")))
	if len(fresh) != 7 {
		t.Fatalf("fresh columns = %v", fresh)
	}

	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(`CREATE TABLE sessions (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, tool TEXT NOT NULL, cwd TEXT NOT NULL,
		group_name TEXT NOT NULL, status TEXT NOT NULL, archived INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL, last_status_at INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	legacy.Close()

	upgraded, err := Open(path)
	if err != nil {
		t.Fatalf("first upgrade: %v", err)
	}
	if err := upgraded.AddAutomation(Automation{ID: "kept", Name: "kept", Config: []byte(`{}`)}, time.Now()); err != nil {
		t.Fatal(err)
	}
	upgraded.Close()
	again := openAutomationStore(t, path)
	if got := automationColumnsOf(t, again); !reflect.DeepEqual(got, fresh) {
		t.Fatalf("upgraded columns = %v, want %v", got, fresh)
	}
	if _, found, err := again.Automation("kept"); !found || err != nil {
		t.Fatalf("second upgrade lost the row: %v %v", found, err)
	}
}
