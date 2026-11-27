package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func sampleBotProfile() BotProfile {
	return BotProfile{
		ID: "p1", Name: "operator", Emoji: "🛠", Color: 3, Tool: "claude", Model: "opus",
		Dir: "/work/fractory", Flags: "--permission-mode plan", Specialization: "fractory-operator",
		MemoryProject: "fractory-operator",
	}
}

func TestBotProfilesRoundTrip(t *testing.T) {
	s := openAutomationStore(t, filepath.Join(t.TempDir(), "state.db"))
	if listed, err := s.BotProfiles(); err != nil || listed == nil || len(listed) != 0 {
		t.Fatalf("empty BotProfiles = %#v, %v; want an empty list", listed, err)
	}
	created := time.Date(2026, 11, 27, 9, 0, 0, 123, time.UTC)
	want := sampleBotProfile()
	if err := s.AddBotProfile(want, created); err != nil {
		t.Fatal(err)
	}
	got, found, err := s.BotProfile("p1")
	if err != nil || !found {
		t.Fatalf("BotProfile = %v, %v", found, err)
	}
	if !got.CreatedAt.Equal(created) || !got.UpdatedAt.Equal(created) {
		t.Fatalf("stamps = %v %v, want %v", got.CreatedAt, got.UpdatedAt, created)
	}
	want.CreatedAt, want.UpdatedAt = got.CreatedAt, got.UpdatedAt
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("read back %+v, want %+v", got, want)
	}

	changed := created.Add(time.Hour)
	edit := want
	edit.Name, edit.Color, edit.Model, edit.Flags = "watcher", 0, "", ""
	if err := s.UpdateBotProfile(edit, changed); err != nil {
		t.Fatal(err)
	}
	got, _, _ = s.BotProfile("p1")
	if !got.CreatedAt.Equal(created) || !got.UpdatedAt.Equal(changed) {
		t.Fatalf("stamps after edit = %v %v", got.CreatedAt, got.UpdatedAt)
	}
	edit.CreatedAt, edit.UpdatedAt = got.CreatedAt, got.UpdatedAt
	if !reflect.DeepEqual(got, edit) {
		t.Fatalf("after edit %+v, want %+v", got, edit)
	}
	listed, err := s.BotProfiles()
	if err != nil || len(listed) != 1 || !reflect.DeepEqual(listed[0], edit) {
		t.Fatalf("BotProfiles = %+v, %v", listed, err)
	}
}

func TestBotProfileIDsAreUniqueAndEditsNeedOne(t *testing.T) {
	s := openAutomationStore(t, filepath.Join(t.TempDir(), "state.db"))
	if err := s.AddBotProfile(sampleBotProfile(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.AddBotProfile(sampleBotProfile(), time.Now()); !errors.Is(err, ErrBotProfileTaken) {
		t.Fatalf("second add = %v, want ErrBotProfileTaken", err)
	}
	missing := sampleBotProfile()
	missing.ID = "nosuch"
	if err := s.UpdateBotProfile(missing, time.Now()); !errors.Is(err, ErrBotProfileNotFound) {
		t.Fatalf("edit of a missing id = %v, want ErrBotProfileNotFound", err)
	}
	if _, found, err := s.BotProfile("nosuch"); found || err != nil {
		t.Fatalf("missing BotProfile = %v, %v", found, err)
	}
}

// Every process opens state.db on its own, so a Profile one writes is the
// Profile another reads.
func TestBotProfileReadsTheSameFromTwoOpeners(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	writer := openAutomationStore(t, path)
	reader := openAutomationStore(t, path)
	if err := writer.AddBotProfile(sampleBotProfile(), time.Now()); err != nil {
		t.Fatal(err)
	}
	fromWriter, _, err := writer.BotProfile("p1")
	if err != nil {
		t.Fatal(err)
	}
	fromReader, found, err := reader.BotProfile("p1")
	if err != nil || !found || !reflect.DeepEqual(fromReader, fromWriter) {
		t.Fatalf("second opener read %+v, %v, %v; want %+v", fromReader, found, err, fromWriter)
	}
}

func botProfileColumnsOf(t *testing.T, s *Store) []string {
	t.Helper()
	rows, err := s.db.Query(`SELECT name, type, "notnull", COALESCE(dflt_value, ''), pk FROM pragma_table_info('bot_profiles') ORDER BY cid`)
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

func TestBotProfilesTableIsTheSameOnFreshAndUpgradedStores(t *testing.T) {
	fresh := botProfileColumnsOf(t, openAutomationStore(t, filepath.Join(t.TempDir(), "fresh.db")))
	if len(fresh) != 12 {
		t.Fatalf("fresh columns = %v", fresh)
	}

	// A database from before Bots: an older release's sessions table and
	// its Automations, with no bot_profiles.
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
	older, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := older.db.Exec(`DROP TABLE bot_profiles`); err != nil {
		t.Fatal(err)
	}
	if err := older.AddAutomation(Automation{ID: "kept", Name: "kept", Config: []byte(`{}`)}, time.Now()); err != nil {
		t.Fatal(err)
	}
	older.Close()

	upgraded, err := Open(path)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if err := upgraded.AddBotProfile(sampleBotProfile(), time.Now()); err != nil {
		t.Fatal(err)
	}
	upgraded.Close()
	again := openAutomationStore(t, path)
	if got := botProfileColumnsOf(t, again); !reflect.DeepEqual(got, fresh) {
		t.Fatalf("upgraded columns = %v, want %v", got, fresh)
	}
	if _, found, err := again.BotProfile("p1"); !found || err != nil {
		t.Fatalf("reopening lost the Profile: %v %v", found, err)
	}
	if _, found, err := again.Automation("kept"); !found || err != nil {
		t.Fatalf("the upgrade lost an Automation: %v %v", found, err)
	}
}
