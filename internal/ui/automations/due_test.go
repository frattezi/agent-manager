package automations

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func automation(id, cron, tz string, enabled bool, lastFired, created time.Time) Stored {
	return Stored{
		Config:  Config{Version: 1, ID: id, Cron: cron, Timezone: tz},
		Enabled: enabled, LastFiredFor: lastFired, CreatedAt: created,
	}
}

func TestDueFiresForTheInstantNotTheMomentNoticed(t *testing.T) {
	a := automation("daily", "0 7 * * *", "UTC", true, at(t, "2026-11-25T07:00:00Z"), at(t, "2026-11-01T00:00:00Z"))
	got := Due([]Stored{a}, nil, at(t, "2026-11-26T07:02:41Z"))
	want := Decision{Fire: []Firing{{ID: "daily", FiredFor: at(t, "2026-11-26T07:00:00Z")}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Due = %+v, want %+v", got, want)
	}
}

func TestDueIsNothingBeforeTheNextInstant(t *testing.T) {
	a := automation("daily", "0 7 * * *", "UTC", true, at(t, "2026-11-26T07:00:00Z"), at(t, "2026-11-01T00:00:00Z"))
	if got := Due([]Stored{a}, nil, at(t, "2026-11-27T06:59:59Z")); !reflect.DeepEqual(got, Decision{}) {
		t.Errorf("Due = %+v, want nothing", got)
	}
}

func TestDueNeverReportsADisabledAutomation(t *testing.T) {
	a := automation("daily", "0 7 * * *", "UTC", false, at(t, "2026-11-01T07:00:00Z"), at(t, "2026-11-01T00:00:00Z"))
	if got := Due([]Stored{a}, map[string]bool{"daily": true}, at(t, "2026-11-26T08:00:00Z")); !reflect.DeepEqual(got, Decision{}) {
		t.Errorf("Due = %+v, want nothing", got)
	}
}

func TestDueWaitsForANewAutomationsNextInstant(t *testing.T) {
	a := automation("hourly", "0 * * * *", "UTC", true, time.Time{}, at(t, "2026-11-26T10:20:00Z"))
	if got := Due([]Stored{a}, nil, at(t, "2026-11-26T10:20:00Z")); !reflect.DeepEqual(got, Decision{}) {
		t.Errorf("Due on sight = %+v, want nothing", got)
	}
	if got := Due([]Stored{a}, nil, at(t, "2026-11-26T10:59:00Z")); !reflect.DeepEqual(got, Decision{}) {
		t.Errorf("Due before its first instant = %+v, want nothing", got)
	}
	got := Due([]Stored{a}, nil, at(t, "2026-11-26T11:00:30Z"))
	want := Decision{Fire: []Firing{{ID: "hourly", FiredFor: at(t, "2026-11-26T11:00:00Z")}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Due at its first instant = %+v, want %+v", got, want)
	}
}

func TestDueCollapsesMissedInstantsIntoTheMostRecent(t *testing.T) {
	a := automation("daily", "0 7 * * *", "UTC", true, at(t, "2026-11-20T07:00:00Z"), at(t, "2026-11-01T00:00:00Z"))
	got := Due([]Stored{a}, nil, at(t, "2026-11-26T06:00:00Z"))
	want := Decision{Fire: []Firing{{ID: "daily", FiredFor: at(t, "2026-11-25T07:00:00Z")}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Due = %+v, want one firing for the latest missed instant %+v", got, want)
	}
}

func TestDueSkipsABusyAutomation(t *testing.T) {
	busy := automation("busy", "0 7 * * *", "UTC", true, at(t, "2026-11-25T07:00:00Z"), at(t, "2026-11-01T00:00:00Z"))
	free := automation("free", "0 7 * * *", "UTC", true, at(t, "2026-11-25T07:00:00Z"), at(t, "2026-11-01T00:00:00Z"))
	got := Due([]Stored{busy, free}, map[string]bool{"busy": true}, at(t, "2026-11-26T07:01:00Z"))
	want := Decision{
		Fire:    []Firing{{ID: "free", FiredFor: at(t, "2026-11-26T07:00:00Z")}},
		Skipped: []Firing{{ID: "busy", FiredFor: at(t, "2026-11-26T07:00:00Z")}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Due = %+v, want %+v", got, want)
	}
}

func TestDueReadsTheCronInTheRecordedTimezone(t *testing.T) {
	time.Local = time.FixedZone("machine", -8*3600)
	t.Cleanup(func() { time.Local = time.UTC })
	a := automation("tokyo", "0 7 * * *", "Asia/Tokyo", true, at(t, "2026-11-25T22:00:00Z"), at(t, "2026-11-01T00:00:00Z"))
	if got := Due([]Stored{a}, nil, at(t, "2026-11-26T07:00:00Z")); !reflect.DeepEqual(got, Decision{}) {
		t.Errorf("Due at 07:00 UTC = %+v, want nothing", got)
	}
	got := Due([]Stored{a}, nil, at(t, "2026-11-26T22:00:00Z"))
	want := Decision{Fire: []Firing{{ID: "tokyo", FiredFor: at(t, "2026-11-26T22:00:00Z")}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Due at 07:00 Tokyo = %+v, want %+v", got, want)
	}
}

func TestDueFiresASkippedLocalInstantOnTheNextEvaluation(t *testing.T) {
	// 01:30 does not exist in Lisbon on 29 March 2026; the clocks go from
	// 01:00 straight to 02:00, which is 01:00 UTC.
	a := automation("night", "30 1 * * *", "Europe/Lisbon", true, at(t, "2026-03-28T01:30:00Z"), at(t, "2026-03-01T00:00:00Z"))
	if got := Due([]Stored{a}, nil, at(t, "2026-03-29T00:59:00Z")); !reflect.DeepEqual(got, Decision{}) {
		t.Errorf("Due before the gap = %+v, want nothing", got)
	}
	got := Due([]Stored{a}, nil, at(t, "2026-03-29T01:01:00Z"))
	want := Decision{Fire: []Firing{{ID: "night", FiredFor: at(t, "2026-03-29T01:00:00Z")}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Due after the gap = %+v, want %+v", got, want)
	}
}

func TestDueFiresARepeatedLocalInstantOnce(t *testing.T) {
	// 01:30 happens twice in Lisbon on 25 October 2026: at 00:30 and 01:30 UTC.
	a := automation("night", "30 1 * * *", "Europe/Lisbon", true, at(t, "2026-10-24T00:30:00Z"), at(t, "2026-10-01T00:00:00Z"))
	got := Due([]Stored{a}, nil, at(t, "2026-10-25T00:31:00Z"))
	want := Decision{Fire: []Firing{{ID: "night", FiredFor: at(t, "2026-10-25T00:30:00Z")}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Due at the first 01:30 = %+v, want %+v", got, want)
	}
	a.LastFiredFor = got.Fire[0].FiredFor
	if got := Due([]Stored{a}, nil, at(t, "2026-10-25T01:31:00Z")); !reflect.DeepEqual(got, Decision{}) {
		t.Errorf("Due at the second 01:30 = %+v, want nothing", got)
	}
}

func TestDueGivesTheSameAnswerForTheSameInputs(t *testing.T) {
	a := []Stored{automation("daily", "0 7 * * *", "UTC", true, at(t, "2026-11-20T07:00:00Z"), at(t, "2026-11-01T00:00:00Z"))}
	now := at(t, "2026-11-26T08:00:00Z")
	first := Due(a, nil, now)
	if second := Due(a, nil, now); !reflect.DeepEqual(first, second) {
		t.Errorf("Due changed between calls: %+v then %+v", first, second)
	}
}

func TestDueReportsAnUnreadableSchedule(t *testing.T) {
	a := automation("broken", "0 7 * *", "UTC", true, time.Time{}, at(t, "2026-11-01T00:00:00Z"))
	got := Due([]Stored{a}, nil, at(t, "2026-11-26T08:00:00Z"))
	if len(got.Fire)+len(got.Skipped) != 0 || len(got.Unreadable) != 1 || got.Unreadable[0].ID != "broken" {
		t.Errorf("Due = %+v, want broken reported unreadable", got)
	}
}

func TestBusyCountsPendingRequestsAndUnfinishedRecords(t *testing.T) {
	config, _ := json.Marshal(Config{ID: "pending"})
	requests := []Request{{Version: 1, Automation: config}}
	records := []Record{
		{AutomationID: "queued", Status: StatusQueued},
		{AutomationID: "running", Status: StatusRunning},
		{AutomationID: "finished", Status: "done"},
	}
	want := map[string]bool{"pending": true, "queued": true, "running": true}
	if got := Busy(requests, records); !reflect.DeepEqual(got, want) {
		t.Errorf("Busy = %v, want %v", got, want)
	}
}
