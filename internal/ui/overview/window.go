// Package overview computes and draws the Overview a section header opens: a
// read-only summary built from records already on disk. Computing reads no
// files, no store and no tmux; drawing takes the value, a width, the glyph
// mode and a palette.
package overview

import "time"

// Window is how far back the Runs Overview looks. It steps 24h, 7d, 30d, all.
type Window int

const (
	WindowAll Window = iota
	Window24h
	Window7d
	Window30d
)

// Next is the window one step on, wrapping from the last to the first.
func (w Window) Next() Window {
	switch w {
	case Window24h:
		return Window7d
	case Window7d:
		return Window30d
	case Window30d:
		return WindowAll
	}
	return Window24h
}

// Label is how the window reads in the Overview.
func (w Window) Label() string {
	switch w {
	case Window24h:
		return "24h"
	case Window7d:
		return "7d"
	case Window30d:
		return "30d"
	}
	return "all"
}

// Span is the window's length; zero for all.
func (w Window) Span() time.Duration {
	switch w {
	case Window24h:
		return 24 * time.Hour
	case Window7d:
		return 7 * 24 * time.Hour
	case Window30d:
		return 30 * 24 * time.Hour
	}
	return 0
}

// Contains reports whether a Run last updated at t is inside the window at now.
func (w Window) Contains(t, now time.Time) bool {
	span := w.Span()
	return span == 0 || !t.Before(now.Add(-span))
}
