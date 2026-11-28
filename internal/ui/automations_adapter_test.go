package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/store"
)

func automationDoc(id string) []byte {
	return []byte(`{"version":1,"id":"` + id + `","name":"nightly","kind":"script","repoPath":"/src/repo","cron":"* * * * *","timezone":"UTC","mode":"work","cli":"","flags":[],"entryPoint":"scripts/nightly.sh","keepRuns":3}`)
}

func addTestAutomation(t *testing.T, m *Model, id string, enabled bool) {
	t.Helper()
	created := time.Now().Add(-time.Hour)
	if err := m.services.store.AddAutomation(store.Automation{ID: id, Name: "nightly", Config: automationDoc(id), Enabled: enabled}, created); err != nil {
		t.Fatal(err)
	}
}

func fractoryHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("FRACTORY_HOME", home)
	return home
}

func TestAutomationsTickFiresADueAutomationOffTheUpdatePath(t *testing.T) {
	home := fractoryHome(t)
	m := buildModel(t)
	addTestAutomation(t, m, "nightly", true)
	requests := filepath.Join(home, "automations", "requests")

	m.applyTestMsg(t, automationsCheckNow())
	if entries, _ := os.ReadDir(requests); len(entries) != 0 {
		t.Fatalf("the tick wrote %d requests on the update path", len(entries))
	}
	m.drainEffects(t)

	entries, err := os.ReadDir(requests)
	if err != nil || len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "nightly@") {
		t.Fatalf("requests = %v, %v; want one for nightly", entries, err)
	}
	a, _, err := m.services.store.Automation("nightly")
	if err != nil {
		t.Fatal(err)
	}
	if a.LastFiredFor.IsZero() || a.LastFiredFor.After(time.Now()) {
		t.Fatalf("watermark = %v; want the instant fired for", a.LastFiredFor)
	}

	m.applyTestMsg(t, automationsCheckNow())
	m.drainEffects(t)
	if again, _ := os.ReadDir(requests); len(again) != 1 {
		t.Fatalf("a second check requested again: %d requests", len(again))
	}
}

func TestAutomationsTickLeavesADisabledAutomation(t *testing.T) {
	home := fractoryHome(t)
	m := buildModel(t)
	addTestAutomation(t, m, "nightly", false)
	m.applyTestMsg(t, automationsCheckNow())
	m.drainEffects(t)
	if entries, _ := os.ReadDir(filepath.Join(home, "automations", "requests")); len(entries) != 0 {
		t.Fatalf("a disabled Automation fired: %v", entries)
	}
}

func TestAutomationsTickReportsAFailedWriteAndKeepsTheWatermark(t *testing.T) {
	fractoryHome(t)
	m := buildModel(t)
	// An id no file can carry makes the request write fail.
	addTestAutomation(t, m, "bad id", true)
	m.applyTestMsg(t, automationsCheckNow())
	m.drainEffects(t)
	if !strings.Contains(m.errBar.text, "Automation bad id did not fire") {
		t.Fatalf("status = %q; want the failed firing reported", m.errBar.text)
	}
	a, _, err := m.services.store.Automation("bad id")
	if err != nil {
		t.Fatal(err)
	}
	if !a.LastFiredFor.IsZero() {
		t.Fatalf("watermark moved to %v", a.LastFiredFor)
	}
}
