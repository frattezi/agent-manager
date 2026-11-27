package automations

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const dailyDoc = `{"version":1,"id":"daily","name":"daily","kind":"script","repoPath":"/src/repo","cron":"0 7 * * *","timezone":"UTC","mode":"work","cli":"","flags":[],"entryPoint":"scripts/daily.sh","keepRuns":5}`

// fakeStore is the store a check runs against. listed, when set, is what
// the listing returns instead of the rows, so a test can change a row
// between the listing and the firing.
type fakeStore struct {
	rows      map[string]Row
	listed    []Row
	listErr   error
	onAdvance func(id string, firedFor time.Time)
	advances  int
}

func (s *fakeStore) AutomationRows() ([]Row, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.listed != nil {
		return s.listed, nil
	}
	var rows []Row
	for _, row := range s.rows {
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *fakeStore) AutomationRow(id string) (Row, bool, error) {
	row, ok := s.rows[id]
	return row, ok, nil
}

func (s *fakeStore) SetLastFired(id string, firedFor time.Time) error {
	if s.onAdvance != nil {
		s.onAdvance(id, firedFor)
	}
	row := s.rows[id]
	row.LastFiredFor = firedFor
	s.rows[id] = row
	s.advances++
	return nil
}

func day(d, hour, minute int) time.Time { return time.Date(2026, 11, d, hour, minute, 0, 0, time.UTC) }

func newFixture(t *testing.T, row Row) (*fakeStore, Services) {
	t.Helper()
	if row.ID == "" {
		row.ID = "daily"
	}
	if row.Document == nil {
		row.Document = []byte(dailyDoc)
	}
	if row.CreatedAt.IsZero() {
		row.CreatedAt = day(1, 6, 0)
	}
	store := &fakeStore{rows: map[string]Row{row.ID: row}}
	root := t.TempDir()
	return store, Services{Store: store, RequestsDir: filepath.Join(root, "automations"), RecordsDir: filepath.Join(root, "automation-runs")}
}

func requestFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

func writeRecord(t *testing.T, dir, status string, firedFor time.Time) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"version":1,"id":"run-1","automationId":"daily","firedFor":"` + firedFor.UTC().Format(time.RFC3339) + `","status":"` + status + `"}`
	if err := os.WriteFile(filepath.Join(dir, "run-1.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckFiresADueAutomationAndAdvancesItsWatermark(t *testing.T) {
	store, services := newFixture(t, Row{Enabled: true})
	result, err := Check(services, day(1, 7, 0).Add(30*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fired) != 1 || !result.Fired[0].FiredFor.Equal(day(1, 7, 0)) {
		t.Fatalf("fired = %+v; want daily at 07:00", result.Fired)
	}
	if got := requestFiles(t, services.RequestsDir); len(got) != 1 || got[0] != "daily@20261101T070000Z.json" {
		t.Fatalf("requests = %v", got)
	}
	if got := store.rows["daily"].LastFiredFor; !got.Equal(day(1, 7, 0)) {
		t.Fatalf("watermark = %v", got)
	}
}

func TestCheckWritesTheRequestBeforeAdvancingTheWatermark(t *testing.T) {
	store, services := newFixture(t, Row{Enabled: true})
	store.onAdvance = func(id string, firedFor time.Time) {
		name, _ := RequestName(id, firedFor)
		if _, err := os.Stat(filepath.Join(services.RequestsDir, name)); err != nil {
			t.Errorf("watermark advanced before the request was written: %v", err)
		}
	}
	if _, err := Check(services, day(1, 8, 0)); err != nil {
		t.Fatal(err)
	}
	if store.advances != 1 {
		t.Fatalf("advances = %d, want 1", store.advances)
	}
}

func TestCheckNeverRequestsAnInstantTwice(t *testing.T) {
	_, services := newFixture(t, Row{Enabled: true})
	for _, now := range []time.Time{day(1, 7, 1), day(1, 7, 3), day(1, 23, 0)} {
		result, err := Check(services, now)
		if err != nil {
			t.Fatal(err)
		}
		if now.After(day(1, 7, 1)) && len(result.Fired) != 0 {
			t.Fatalf("check at %v fired %+v again", now, result.Fired)
		}
	}
	if got := requestFiles(t, services.RequestsDir); len(got) != 1 {
		t.Fatalf("requests = %v, want one", got)
	}
}

func TestCheckFiresAMissedInstantOnceOnStartup(t *testing.T) {
	store, services := newFixture(t, Row{Enabled: true, LastFiredFor: day(1, 7, 0)})
	result, err := Check(services, day(4, 9, 15))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fired) != 1 || !result.Fired[0].FiredFor.Equal(day(4, 7, 0)) {
		t.Fatalf("fired = %+v; want only the latest missed instant", result.Fired)
	}
	if got := requestFiles(t, services.RequestsDir); len(got) != 1 || got[0] != "daily@20261104T070000Z.json" {
		t.Fatalf("requests = %v", got)
	}
	if got := store.rows["daily"].LastFiredFor; !got.Equal(day(4, 7, 0)) {
		t.Fatalf("watermark = %v", got)
	}
}

func TestCheckSkipsABusyAutomationAndDoesNotRetryTheInstant(t *testing.T) {
	store, services := newFixture(t, Row{Enabled: true, LastFiredFor: day(1, 7, 0)})
	writeRecord(t, services.RecordsDir, StatusRunning, day(1, 7, 0))
	result, err := Check(services, day(2, 7, 5))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fired) != 0 || len(result.Skipped) != 1 || !result.Skipped[0].FiredFor.Equal(day(2, 7, 0)) {
		t.Fatalf("result = %+v; want 2 Nov 07:00 skipped", result)
	}
	if got := store.rows["daily"].LastFiredFor; !got.Equal(day(2, 7, 0)) {
		t.Fatalf("watermark = %v; want the skipped instant", got)
	}
	writeRecord(t, services.RecordsDir, "done", day(1, 7, 0))
	result, err = Check(services, day(2, 8, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fired)+len(result.Skipped) != 0 || requestFiles(t, services.RequestsDir) != nil {
		t.Fatalf("skipped instant was retried: %+v, %v", result, requestFiles(t, services.RequestsDir))
	}
}

func TestCheckAfterACrashBetweenWriteAndAdvanceDoesNotRequestAgain(t *testing.T) {
	t.Run("request still outstanding", func(t *testing.T) {
		store, services := newFixture(t, Row{Enabled: true})
		if _, err := WriteRequest(services.RequestsDir, []byte(dailyDoc), day(1, 7, 0), day(1, 7, 0)); err != nil {
			t.Fatal(err)
		}
		before := requestFiles(t, services.RequestsDir)
		result, err := Check(services, day(1, 7, 2))
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Fired) != 0 || !store.rows["daily"].LastFiredFor.Equal(day(1, 7, 0)) {
			t.Fatalf("result = %+v, watermark = %v", result, store.rows["daily"].LastFiredFor)
		}
		if after := requestFiles(t, services.RequestsDir); len(after) != len(before) {
			t.Fatalf("requests = %v, want %v", after, before)
		}
	})
	t.Run("request already run", func(t *testing.T) {
		store, services := newFixture(t, Row{Enabled: true})
		writeRecord(t, services.RecordsDir, "done", day(1, 7, 0))
		result, err := Check(services, day(1, 7, 2))
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Fired) != 0 || requestFiles(t, services.RequestsDir) != nil {
			t.Fatalf("an instant Fractory already ran was requested again: %+v", result)
		}
		if got := store.rows["daily"].LastFiredFor; !got.Equal(day(1, 7, 0)) {
			t.Fatalf("watermark = %v", got)
		}
	})
}

func TestCheckLeavesTheWatermarkWhenTheRequestCannotBeWritten(t *testing.T) {
	// An id no file can carry makes every request write fail.
	store, services := newFixture(t, Row{ID: "bad id", Enabled: true, Document: []byte(strings.Replace(dailyDoc, `"id":"daily"`, `"id":"bad id"`, 1))})
	result, err := Check(services, day(1, 7, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Failed) != 1 || result.Failed[0].ID != "bad id" || len(result.Fired) != 0 {
		t.Fatalf("result = %+v; want the firing reported failed", result)
	}
	if !store.rows["bad id"].LastFiredFor.IsZero() || store.advances != 0 {
		t.Fatalf("watermark moved to %v", store.rows["bad id"].LastFiredFor)
	}
	if again, _ := Check(services, day(1, 7, 2)); len(again.Failed) != 1 {
		t.Fatalf("the next check did not try again: %+v", again)
	}
}

func TestCheckWithoutAFractoryHomeReportsTheFiring(t *testing.T) {
	store, services := newFixture(t, Row{Enabled: true})
	services.RequestsDir, services.RecordsDir = "", ""
	result, err := Check(services, day(1, 7, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Failed) != 1 || store.advances != 0 {
		t.Fatalf("result = %+v, advances = %d", result, store.advances)
	}
}

func TestCheckNeverFiresADisabledAutomation(t *testing.T) {
	store, services := newFixture(t, Row{Enabled: false})
	result, err := Check(services, day(3, 7, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fired)+len(result.Skipped)+len(result.Failed) != 0 || store.advances != 0 || requestFiles(t, services.RequestsDir) != nil {
		t.Fatalf("disabled Automation acted on: %+v", result)
	}
}

func TestCheckHonoursADisableBetweenListingAndFiring(t *testing.T) {
	store, services := newFixture(t, Row{Enabled: true})
	listed := store.rows["daily"]
	store.listed = []Row{listed}
	disabled := listed
	disabled.Enabled = false
	store.rows["daily"] = disabled
	result, err := Check(services, day(1, 7, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Fired) != 0 || store.advances != 0 || requestFiles(t, services.RequestsDir) != nil {
		t.Fatalf("Automation disabled mid-cycle fired: %+v", result)
	}
}

func TestCheckReportsAStoreItCannotRead(t *testing.T) {
	store, services := newFixture(t, Row{Enabled: true})
	store.listErr = errors.New("database is locked")
	if _, err := Check(services, day(1, 7, 1)); err == nil {
		t.Fatal("want the store error")
	}
}
