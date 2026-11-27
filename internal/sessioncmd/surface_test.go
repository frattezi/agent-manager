package sessioncmd

import (
	"strings"
	"testing"
	"time"
)

func TestTheListReadsEachAnsweredRoundBackWithItsAnswer(t *testing.T) {
	at := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)
	answered := at.Add(time.Minute)
	got := FormatSurfaceList([]AgentSurface{
		{ID: "a", SurfaceID: "round-1", PublishedAt: at, Answered: true, AnsweredAt: &answered, Answer: "Pick\n  A (the recommended answer)"},
		{ID: "b", SurfaceID: "round-2", PublishedAt: at.Add(time.Hour)},
	})
	want := strings.Join([]string{
		"round-1 (a) [answered] published 2026-11-27T09:00:00Z",
		"  Pick",
		"    A (the recommended answer)",
		"round-2 (b) [unanswered] published 2026-11-27T10:00:00Z",
	}, "\n")
	if got != want {
		t.Fatalf("list =\n%s\nwant\n%s", got, want)
	}
}
