package overview

import (
	"slices"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/ui/automations"
)

func entry(id, repo, cron string, enabled bool, firings ...automations.Record) automations.Entry {
	return automations.Entry{ID: id, Name: id, Enabled: enabled, Config: automations.Config{ID: id, RepoPath: repo, Cron: cron, Timezone: "UTC"}, Firings: firings}
}

func firing(id, status string, ago time.Duration, summary string) automations.Record {
	r := automations.Record{ID: id, Status: status, FiredFor: now.Add(-ago)}
	if summary != "" {
		r.Result = &automations.Result{Outcome: status, Summary: summary}
	}
	return r
}

func groupNames(o Automations) []string {
	var out []string
	for _, g := range o.Groups {
		out = append(out, g.Name)
	}
	return out
}

func TestAutomationsGroupByProjectWithElsewhereLast(t *testing.T) {
	entries := []automations.Entry{
		entry("loose", "", "0 * * * *", true, firing("l1", "done", 30*time.Minute, "")),
		entry("old", "/src/web", "0 * * * *", true, firing("o1", "done", 3*24*time.Hour, "")),
		entry("new", "/src/api", "0 * * * *", true, firing("n1", "done", time.Hour, "")),
		entry("api2", "/other/api", "0 * * * *", true),
	}
	got := ComputeAutomations(entries, WindowAll, now)
	if want := []string{"api", "web", Elsewhere}; !slices.Equal(groupNames(got), want) {
		t.Fatalf("groups = %v; want %v", groupNames(got), want)
	}
	if n := len(got.Groups[0].Automations); n != 2 {
		t.Fatalf("api group has %d Automations", n)
	}
}

func TestAutomationsUpcomingFiresWithinTwentyFourHours(t *testing.T) {
	// now is 12:30 UTC: the hourly schedule fires 13:00 through 12:00 next day.
	got := ComputeAutomations([]automations.Entry{
		entry("hourly", "/src/a", "0 * * * *", true),
		entry("daily", "/src/a", "0 13 * * *", true),
		entry("later", "/src/a", "45 12 * * *", true),
		entry("yearly", "/src/a", "0 7 1 1 *", true),
		entry("off", "/src/a", "0 * * * *", false),
	}, WindowAll, now)
	byName := map[string]Automation{}
	for _, a := range got.Groups[0].Automations {
		byName[a.Name] = a
	}
	if n := len(byName["hourly"].Upcoming); n != 24 {
		t.Errorf("hourly has %d upcoming", n)
	}
	if up := byName["hourly"].Upcoming; up[0] != 30*time.Minute || up[23] != 23*time.Hour+30*time.Minute {
		t.Errorf("hourly offsets = %v", up)
	}
	if up := byName["daily"].Upcoming; len(up) != 1 || up[0] != 30*time.Minute {
		t.Errorf("daily = %v", up)
	}
	if up := byName["later"].Upcoming; len(up) != 1 || up[0] != 15*time.Minute {
		t.Errorf("later = %v", up)
	}
	if n := len(byName["yearly"].Upcoming); n != 0 {
		t.Errorf("yearly has %d upcoming", n)
	}
	if off := byName["off"]; off.Enabled || len(off.Upcoming) != 0 {
		t.Errorf("a disabled Automation has upcoming fires: %+v", off)
	}
}

func TestAutomationsLastTenOutcomesOldestFirstAndWindow(t *testing.T) {
	var records []automations.Record
	for i := range automations.RecentFirings {
		status := "done"
		if i == 0 {
			status = "failed"
		}
		records = append(records, firing("r", status, time.Duration(i+1)*time.Hour*10, "summary "+status))
	}
	entries := []automations.Entry{entry("a", "/src/a", "0 7 1 1 *", true, records...)}
	row := ComputeAutomations(entries, WindowAll, now).Groups[0].Automations[0]
	if len(row.Firings) != 10 || row.Firings[9].Status != "failed" || row.Firings[0].Status != "done" {
		t.Fatalf("firings = %+v", row.Firings)
	}
	if !row.HasLatest || row.Latest.Status != "failed" || row.LatestSummary != "summary failed" {
		t.Fatalf("latest = %+v %q", row.Latest, row.LatestSummary)
	}
	// 10h, 20h inside 24h (offsets 10h, 20h); the rest are older.
	row = ComputeAutomations(entries, Window24h, now).Groups[0].Automations[0]
	if len(row.Firings) != 2 || row.Firings[1].Status != "failed" {
		t.Fatalf("24h firings = %+v", row.Firings)
	}
	row = ComputeAutomations(entries, Window24h, now.Add(10*24*time.Hour)).Groups[0].Automations[0]
	if len(row.Firings) != 0 || row.HasLatest {
		t.Fatalf("a window past every firing still shows %+v", row)
	}
}

func TestAutomationsUnreadableLandsElsewhere(t *testing.T) {
	bad := automations.Entry{ID: "bad", Name: "bad", Enabled: true, Err: errTest{}}
	got := ComputeAutomations([]automations.Entry{bad}, WindowAll, now)
	if len(got.Groups) != 1 || got.Groups[0].Name != Elsewhere || !got.Groups[0].Automations[0].Unreadable || len(got.Groups[0].Automations[0].Upcoming) != 0 {
		t.Fatalf("groups = %+v", got.Groups)
	}
}

type errTest struct{}

func (errTest) Error() string { return "bad json" }
