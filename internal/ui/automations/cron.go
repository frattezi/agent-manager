package automations

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

type cronField struct {
	name     string
	min, max int
	names    []string
}

var cronFields = []cronField{
	{name: "minute", min: 0, max: 59},
	{name: "hour", min: 0, max: 23},
	{name: "day of month", min: 1, max: 31},
	{name: "month", min: 1, max: 12, names: []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}},
	// 7 is Sunday as well as 0, as in every cron.
	{name: "day of week", min: 0, max: 7, names: []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}},
}

// Schedule is a cron expression read in one timezone.
type Schedule struct {
	minute, hour, dom, month, dow [64]bool
	// Cron matches either day field when both are restricted, and both
	// when one of them is *.
	domAny, dowAny bool
	loc            *time.Location
}

// ValidateCron checks a five-field cron expression: minute, hour, day of
// month, month and day of week, each a list of *, values, ranges and steps.
func ValidateCron(expr string) error {
	_, err := parseCron(expr)
	return err
}

// ParseSchedule reads expr in the IANA timezone tz.
func ParseSchedule(expr, tz string) (Schedule, error) {
	s, err := parseCron(expr)
	if err != nil {
		return Schedule{}, fmt.Errorf("cron %q %v", expr, err)
	}
	if tz == "" || tz == "Local" {
		return Schedule{}, fmt.Errorf("timezone %q is not an IANA name", tz)
	}
	if s.loc, err = time.LoadLocation(tz); err != nil {
		return Schedule{}, fmt.Errorf("timezone %q is not an IANA name", tz)
	}
	return s, nil
}

func parseCron(expr string) (Schedule, error) {
	fields := strings.Fields(expr)
	if len(fields) != len(cronFields) {
		return Schedule{}, fmt.Errorf("has %d fields, want 5: minute hour day-of-month month day-of-week", len(fields))
	}
	var s Schedule
	sets := []*[64]bool{&s.minute, &s.hour, &s.dom, &s.month, &s.dow}
	for i, field := range fields {
		for _, part := range strings.Split(field, ",") {
			if err := cronFields[i].fill(part, sets[i]); err != nil {
				return Schedule{}, fmt.Errorf("%s field %q %v", cronFields[i].name, field, err)
			}
		}
	}
	s.dow[0] = s.dow[0] || s.dow[7]
	s.domAny = strings.HasPrefix(fields[2], "*")
	s.dowAny = strings.HasPrefix(fields[4], "*")
	return s, nil
}

func (f cronField) fill(part string, set *[64]bool) error {
	span, stepText, stepped := strings.Cut(part, "/")
	step := 1
	if stepped {
		n, err := strconv.Atoi(stepText)
		if err != nil || n < 1 {
			return fmt.Errorf("has step %q, want a whole number above 0", stepText)
		}
		step = n
	}
	from, to := f.min, f.max
	if span != "*" {
		low, high, ranged := strings.Cut(span, "-")
		var err error
		if from, err = f.value(low); err != nil {
			return err
		}
		switch {
		case ranged:
			if to, err = f.value(high); err != nil {
				return err
			}
			if from > to {
				return fmt.Errorf("has range %s, which runs backwards", span)
			}
		case !stepped:
			to = from
		}
	}
	for v := from; v <= to; v += step {
		set[v] = true
	}
	return nil
}

func (f cronField) value(text string) (int, error) {
	for i, name := range f.names {
		if strings.EqualFold(text, name) {
			return i + f.min, nil
		}
	}
	n, err := strconv.Atoi(text)
	if err != nil || n < f.min || n > f.max {
		return 0, fmt.Errorf("has %q, want %d-%d", text, f.min, f.max)
	}
	return n, nil
}

// searchDays bounds a search for a matching day: long enough for a
// schedule on 29 February restricted to one weekday, which recurs every 28
// years, and short of looping forever on 31 February.
const searchDays = 29 * 366

// Next is the first instant of the schedule strictly after after, in UTC,
// and false when the schedule names no day that exists.
func (s Schedule) Next(after time.Time) (time.Time, bool) {
	y, m, d := after.In(s.loc).Date()
	for i := range searchDays {
		for _, t := range s.day(y, m, d+i) {
			if t.After(after) {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

// Latest is the last instant of the schedule at or before at.
func (s Schedule) Latest(at time.Time) (time.Time, bool) {
	y, m, d := at.In(s.loc).Date()
	for i := range searchDays {
		instants := s.day(y, m, d-i)
		for j := len(instants) - 1; j >= 0; j-- {
			if !instants[j].After(at) {
				return instants[j], true
			}
		}
	}
	return time.Time{}, false
}

// day lists the schedule's instants on one local date, earliest first. A
// wall time the clocks skip over is the instant the gap ends; one they
// repeat is its first occurrence only.
func (s Schedule) day(y int, m time.Month, d int) []time.Time {
	date := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	if !s.month[date.Month()] || !s.matchesDay(date) {
		return nil
	}
	var instants []time.Time
	for h := range 24 {
		if !s.hour[h] {
			continue
		}
		for min := range 60 {
			if s.minute[min] {
				instants = append(instants, s.instant(date.Add(time.Duration(h)*time.Hour+time.Duration(min)*time.Minute)))
			}
		}
	}
	slices.SortFunc(instants, time.Time.Compare)
	return slices.CompactFunc(instants, time.Time.Equal)
}

func (s Schedule) matchesDay(date time.Time) bool {
	dom, dow := s.dom[date.Day()], s.dow[date.Weekday()]
	if s.domAny || s.dowAny {
		return dom && dow
	}
	return dom || dow
}

// instant resolves wall, a local wall time written as UTC, in the
// schedule's timezone.
func (s Schedule) instant(wall time.Time) time.Time {
	_, before := wall.Add(-24 * time.Hour).In(s.loc).Zone()
	_, after := wall.Add(24 * time.Hour).In(s.loc).Zone()
	early, late := wall.Add(-time.Duration(max(before, after))*time.Second), wall.Add(-time.Duration(min(before, after))*time.Second)
	for _, t := range []time.Time{early, late} {
		if s.wallOf(t).Equal(wall) {
			return t.UTC()
		}
	}
	// No instant shows this wall time: the clocks jumped over it, at the
	// end of the zone early is still in.
	_, end := early.In(s.loc).ZoneBounds()
	return end.UTC()
}

func (s Schedule) wallOf(t time.Time) time.Time {
	local := t.In(s.loc)
	return time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), local.Minute(), local.Second(), 0, time.UTC)
}
