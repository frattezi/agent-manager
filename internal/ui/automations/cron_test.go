package automations

import (
	"strings"
	"testing"
	"time"
)

func TestValidateCronRefusesNamingTheField(t *testing.T) {
	for _, c := range []struct{ expr, want string }{
		{"", "has 0 fields"},
		{"* * * *", "has 4 fields"},
		{"* * * * * *", "has 6 fields"},
		{"60 * * * *", `minute field "60"`},
		{"-1 * * * *", `minute field "-1"`},
		{"* 24 * * *", `hour field "24"`},
		{"* * 0 * *", `day of month field "0"`},
		{"* * 32 * *", `day of month field "32"`},
		{"* * * 13 *", `month field "13"`},
		{"* * * foo *", `month field "foo"`},
		{"* * * * 8", `day of week field "8"`},
		{"* * * * funday", `day of week field "funday"`},
		{"0 9-7 * * *", "runs backwards"},
		{"*/0 * * * *", `has step "0"`},
		{"*/x * * * *", `has step "x"`},
		{"1,,2 * * * *", `minute field "1,,2"`},
		{"a-5 * * * *", `minute field "a-5"`},
	} {
		err := ValidateCron(c.expr)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("ValidateCron(%q) = %v, want it to mention %q", c.expr, err, c.want)
		}
	}
}

func TestValidateCronReadsNamesInAnyCase(t *testing.T) {
	for _, expr := range []string{"0 0 * JAN-Mar Sun", "0 0 * * mon,WED,fri", "0 0 * dec *"} {
		if err := ValidateCron(expr); err != nil {
			t.Errorf("ValidateCron(%q) = %v", expr, err)
		}
	}
}

func at(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func schedule(t *testing.T, expr, tz string) Schedule {
	t.Helper()
	s, err := ParseSchedule(expr, tz)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNextFollowsTheExpression(t *testing.T) {
	for _, c := range []struct{ expr, after, want string }{
		{"* * * * *", "2026-11-26T10:00:00Z", "2026-11-26T10:01:00Z"},
		{"* * * * *", "2026-11-26T10:00:30Z", "2026-11-26T10:01:00Z"},
		{"*/15 * * * *", "2026-11-26T10:07:00Z", "2026-11-26T10:15:00Z"},
		{"5/20 * * * *", "2026-11-26T10:26:00Z", "2026-11-26T10:45:00Z"},
		{"0-30/10 8-9 * * *", "2026-11-26T08:31:00Z", "2026-11-26T09:00:00Z"},
		{"0 7 * * *", "2026-11-26T07:00:00Z", "2026-11-27T07:00:00Z"},
		{"0 7 * * MON-FRI", "2026-11-27T08:00:00Z", "2026-11-30T07:00:00Z"},
		{"0 0 * * 7", "2026-11-26T00:00:00Z", "2026-11-29T00:00:00Z"},
		{"0 0 * * 0", "2026-11-26T00:00:00Z", "2026-11-29T00:00:00Z"},
		{"0 0 1,15 * *", "2026-11-26T00:00:00Z", "2026-12-01T00:00:00Z"},
		{"30 6 * jan-mar sun", "2026-11-26T00:00:00Z", "2027-01-03T06:30:00Z"},
		{"0 0 29 2 *", "2026-11-26T00:00:00Z", "2028-02-29T00:00:00Z"},
		{"0 0 31 * *", "2026-11-26T00:00:00Z", "2026-12-31T00:00:00Z"},
		// Both day fields restricted: either one matches.
		{"0 0 13 * fri", "2026-11-26T00:00:00Z", "2026-11-27T00:00:00Z"},
		// Day of week left as *: the day of month alone decides.
		{"0 0 13 * *", "2026-11-26T00:00:00Z", "2026-12-13T00:00:00Z"},
	} {
		got, ok := schedule(t, c.expr, "UTC").Next(at(t, c.after))
		if !ok || !got.Equal(at(t, c.want)) {
			t.Errorf("Next(%q, %s) = %v, %v, want %s", c.expr, c.after, got, ok, c.want)
		}
	}
}

func TestNextFindsNothingOnADayThatNeverExists(t *testing.T) {
	if got, ok := schedule(t, "0 0 31 2 *", "UTC").Next(at(t, "2026-11-26T00:00:00Z")); ok {
		t.Errorf("Next = %v, want none", got)
	}
}

func TestLatestIsTheInstantAtOrBefore(t *testing.T) {
	s := schedule(t, "0 7 * * *", "UTC")
	for _, c := range []struct{ at, want string }{
		{"2026-11-26T07:00:00Z", "2026-11-26T07:00:00Z"},
		{"2026-11-26T06:59:59Z", "2026-11-25T07:00:00Z"},
		{"2026-11-26T23:00:00Z", "2026-11-26T07:00:00Z"},
	} {
		got, ok := s.Latest(at(t, c.at))
		if !ok || !got.Equal(at(t, c.want)) {
			t.Errorf("Latest(%s) = %v, %v, want %s", c.at, got, ok, c.want)
		}
	}
}

func TestNextReadsTheScheduleInItsTimezone(t *testing.T) {
	got, _ := schedule(t, "0 7 * * *", "Asia/Tokyo").Next(at(t, "2026-11-26T00:00:00Z"))
	if want := at(t, "2026-11-26T22:00:00Z"); !got.Equal(want) {
		t.Errorf("Next = %v, want %v (07:00 in Tokyo)", got, want)
	}
}

// Lisbon springs forward from 01:00 to 02:00 on 29 March 2026, and falls
// back from 02:00 to 01:00 on 25 October 2026.

func TestNextFiresASkippedWallTimeWhenTheGapEnds(t *testing.T) {
	s := schedule(t, "30 1 * * *", "Europe/Lisbon")
	got, _ := s.Next(at(t, "2026-03-28T01:30:00Z"))
	if want := at(t, "2026-03-29T01:00:00Z"); !got.Equal(want) {
		t.Errorf("Next = %v, want %v, the end of the gap", got, want)
	}
	got, _ = s.Next(got)
	if want := at(t, "2026-03-30T00:30:00Z"); !got.Equal(want) {
		t.Errorf("Next after the gap = %v, want %v", got, want)
	}
}

func TestNextCollapsesEveryInstantInAGapIntoOne(t *testing.T) {
	s := schedule(t, "*/15 1 * * *", "Europe/Lisbon")
	got, _ := s.Next(at(t, "2026-03-28T01:45:00Z"))
	if want := at(t, "2026-03-29T01:00:00Z"); !got.Equal(want) {
		t.Errorf("Next = %v, want %v", got, want)
	}
	got, _ = s.Next(got)
	if want := at(t, "2026-03-30T00:00:00Z"); !got.Equal(want) {
		t.Errorf("Next after the gap = %v, want %v", got, want)
	}
}

func TestNextFiresARepeatedWallTimeOnce(t *testing.T) {
	s := schedule(t, "30 1 * * *", "Europe/Lisbon")
	first, _ := s.Next(at(t, "2026-10-24T00:30:00Z"))
	if want := at(t, "2026-10-25T00:30:00Z"); !first.Equal(want) {
		t.Fatalf("Next = %v, want %v, the first 01:30", first, want)
	}
	second, _ := s.Next(first)
	if want := at(t, "2026-10-26T01:30:00Z"); !second.Equal(want) {
		t.Errorf("Next after the first 01:30 = %v, want %v, the next day", second, want)
	}
}

func TestParseScheduleRefusesAnUnknownTimezone(t *testing.T) {
	for _, tz := range []string{"", "Local", "Mars/Olympus"} {
		if _, err := ParseSchedule("* * * * *", tz); err == nil {
			t.Errorf("ParseSchedule(%q) accepted", tz)
		}
	}
}
