package automations

import (
	"fmt"
	"strconv"
	"strings"
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

// ValidateCron checks a five-field cron expression: minute, hour, day of
// month, month and day of week, each a list of *, values, ranges and steps.
func ValidateCron(expr string) error {
	fields := strings.Fields(expr)
	if len(fields) != len(cronFields) {
		return fmt.Errorf("has %d fields, want 5: minute hour day-of-month month day-of-week", len(fields))
	}
	for i, field := range fields {
		for _, part := range strings.Split(field, ",") {
			if err := cronFields[i].check(part); err != nil {
				return fmt.Errorf("%s field %q %v", cronFields[i].name, field, err)
			}
		}
	}
	return nil
}

func (f cronField) check(part string) error {
	span, step, stepped := strings.Cut(part, "/")
	if stepped {
		n, err := strconv.Atoi(step)
		if err != nil || n < 1 {
			return fmt.Errorf("has step %q, want a whole number above 0", step)
		}
	}
	if span == "*" {
		return nil
	}
	low, high, ranged := strings.Cut(span, "-")
	from, err := f.value(low)
	if err != nil {
		return err
	}
	if !ranged {
		return nil
	}
	to, err := f.value(high)
	if err != nil {
		return err
	}
	if from > to {
		return fmt.Errorf("has range %s, which runs backwards", span)
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
