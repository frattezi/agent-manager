package presentation

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

func TestPadRightUsesDisplayWidthAndKeepsStyledRowsClosed(t *testing.T) {
	styled := "\x1b[1m界\x1b[0m"
	got := PadRight(styled, 4)
	if width := ansi.StringWidth(got); width != 4 {
		t.Fatalf("display width = %d, want 4: %q", width, got)
	}
	if !strings.HasSuffix(got, "\x1b[0m") {
		t.Fatalf("styled padded row did not close its SGR state: %q", got)
	}
	if got := ansi.Strip(PadRight("abcdef", 4)); got != "abc…" {
		t.Fatalf("truncated row = %q, want %q", got, "abc…")
	}
}

func TestDividerUsesCallerStylesAndExactWidth(t *testing.T) {
	section := lipgloss.NewStyle().Bold(true)
	rule := lipgloss.NewStyle().Underline(true)
	got := Divider("list", 20, section, rule, ChromeFor(DefaultGlyphMode).SectionGlyph)
	if width := ansi.StringWidth(got); width != 20 {
		t.Fatalf("divider width = %d, want 20: %q", width, got)
	}
	if plain := ansi.Strip(got); !strings.HasPrefix(plain, "▍list ") {
		t.Fatalf("divider = %q", plain)
	}
}

func TestWindowClampsAndMarksHiddenContent(t *testing.T) {
	lines := []string{"zero", "one", "two", "three", "four"}
	style := lipgloss.NewStyle().Bold(true)

	top := Window(lines, 3, -10, style, DefaultGlyphMode)
	if got := ansi.Strip(strings.Join(top, "\n")); got != "zero\none\n↓ more below…" {
		t.Fatalf("top window:\n%s", got)
	}

	bottom := Window(lines, 3, 99, style, DefaultGlyphMode)
	if got := ansi.Strip(strings.Join(bottom, "\n")); got != "↑ more above…\nthree\nfour" {
		t.Fatalf("bottom window:\n%s", got)
	}

	middle := Window(lines, 1, 2, style, DefaultGlyphMode)
	if got := ansi.Strip(strings.Join(middle, "\n")); got != "↕ more…" {
		t.Fatalf("one-row window = %q", got)
	}
}

func TestTruncateTailRuneSafe(t *testing.T) {
	hebrew := "/home/dev/פרויקטים/agent-manager"
	tail := TruncateTail(hebrew, 10)
	if !strings.HasPrefix(tail, "…") || len([]rune(tail)) != 10 {
		t.Fatalf("TruncateTail broken: %q (%d runes)", tail, len([]rune(tail)))
	}
	if TruncateTail("short", 10) != "short" {
		t.Fatal("short strings should pass through")
	}
	if got := TruncateTail("abcdef", 1); got != "…" {
		t.Fatalf("max 1 = %q, want ellipsis", got)
	}
	if got := TruncateTail("abcdef", 0); got != "" {
		t.Fatalf("max 0 = %q, want empty", got)
	}
}

func TestTruncatePathCutsAtSeparator(t *testing.T) {
	path := "internal/api/handlers/sessions.go"
	got := TruncatePath(path, 16)
	if !strings.HasPrefix(got, "…/") || !strings.HasSuffix(got, "sessions.go") {
		t.Fatalf("TruncatePath = %q, want a slash-cut tail ending in sessions.go", got)
	}
	if strings.Contains(got, "ternal") {
		t.Fatalf("cut mid-segment: %q", got)
	}
	if TruncatePath("short.go", 20) != "short.go" {
		t.Fatal("short paths should pass through")
	}
	if got := TruncatePath(path, 1); got != "…" {
		t.Fatalf("limit 1 = %q, want ellipsis", got)
	}
	if got := TruncatePath(path, 0); got != "" {
		t.Fatalf("limit 0 = %q, want empty", got)
	}
}

func TestFollowHoldsTheWindowWhileTheCursorIsInside(t *testing.T) {
	for _, tc := range []struct {
		first, cursor, count, visible, want int
	}{
		{0, 3, 10, 10, 0},
		{2, 4, 20, 5, 2},
		{2, 6, 20, 5, 2},
		{2, 7, 20, 5, 3},
		{2, 1, 20, 5, 1},
		{2, 19, 20, 5, 15},
		{18, 0, 20, 5, 0},
		{19, 17, 20, 5, 15},
	} {
		if got := Follow(tc.first, tc.cursor, tc.count, tc.visible); got != tc.want {
			t.Errorf("Follow(%d, %d, %d, %d) = %d, want %d", tc.first, tc.cursor, tc.count, tc.visible, got, tc.want)
		}
	}
}

func TestFillSetsTheBackgroundAgainAfterEveryReset(t *testing.T) {
	previous := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
	line := "\x1b[1mname\x1b[0m meta \x1b[2mdim\x1b[0m"

	lipgloss.SetColorProfile(termenv.TrueColor)
	fill := "\x1b[48;2;35;40;48m"
	want := fill + "\x1b[1mname\x1b[0m" + fill + " meta \x1b[2mdim\x1b[0m" + fill + "\x1b[0m"
	if got := Fill(line, "#232830"); got != want {
		t.Fatalf("filled line = %q, want %q", got, want)
	}

	lipgloss.SetColorProfile(termenv.Ascii)
	if got := Fill(line, "#232830"); got != line {
		t.Fatalf("a profile with no colors changed the line to %q", got)
	}
}

func TestWindowDirectionsKeepBoundsAndInput(t *testing.T) {
	input := []string{"literal ↑↓←→", "one", "two", "three", "four"}
	for _, mode := range []GlyphMode{GlyphModeBasic, GlyphModeAdvanced, ""} {
		for _, tc := range []struct {
			rows, offset int
			want         string
		}{
			{3, -10, "literal ↑↓←→\none\n%s more below…"},
			{3, 99, "%s more above…\nthree\nfour"}, {1, 2, "%s more…"},
		} {
			g := NavigationFor(mode)
			mark := g.DownGlyph
			if tc.offset == 99 {
				mark = g.UpGlyph
			}
			if tc.rows == 1 {
				mark = g.UpDownGlyph
			}
			got := ansi.Strip(strings.Join(Window(input, tc.rows, tc.offset, lipgloss.NewStyle(), mode), "\n"))
			if want := fmt.Sprintf(tc.want, mark); got != want {
				t.Fatalf("%s: %q != %q", mode, got, want)
			}
		}
	}
	if input[0] != "literal ↑↓←→" || input[2] != "two" {
		t.Fatal("window mutated input")
	}
	for _, tc := range []struct {
		above           bool
		basic, advanced string
	}{{true, "3 more above", "↑ 3 more"}, {false, "3 more below", "↓ 3 more"}} {
		if got := OverflowLabel(GlyphModeBasic, 3, tc.above); got != tc.basic {
			t.Fatal(got)
		}
		if got := OverflowLabel(GlyphModeAdvanced, 3, tc.above); got != tc.advanced {
			t.Fatal(got)
		}
	}
}

// Reveal is checked against what Window draws, so a row it promises is never
// one an indicator covers.
func TestRevealShowsTheWholeRangeClearOfTheIndicators(t *testing.T) {
	const count, rows = 20, 6
	lines := make([]string, count)
	for i := range lines {
		lines[i] = fmt.Sprintf("row %d", i)
	}
	drawn := func(offset int) string {
		return strings.Join(Window(lines, rows, offset, lipgloss.NewStyle(), DefaultGlyphMode), "\n")
	}
	for offset := 0; offset <= count-rows; offset++ {
		for first := 0; first < count; first++ {
			for last := first; last < count && last-first < 3; last++ {
				got := Reveal(offset, first, last, count, rows)
				view := drawn(got)
				for row := first; row <= last; row++ {
					if !strings.Contains(view, fmt.Sprintf("row %d\n", row)) && !strings.HasSuffix(view, fmt.Sprintf("row %d", row)) {
						t.Fatalf("Reveal(%d, %d, %d) = %d hides row %d:\n%s", offset, first, last, got, row, view)
					}
				}
				if top, bottom := Shown(offset, count, rows); first >= top && last <= bottom && got != offset {
					t.Fatalf("Reveal(%d, %d, %d) scrolled a range already shown to %d", offset, first, last, got)
				}
			}
		}
	}
}

func TestRevealKeepsTheEndOfARangeTallerThanTheWindow(t *testing.T) {
	if got := Reveal(0, 5, 15, 20, 6); got != 11 {
		t.Fatalf("offset = %d, want 11 so row 15 sits above more below", got)
	}
	if top, bottom := Shown(11, 20, 6); top != 12 || bottom != 15 {
		t.Fatalf("shown = %d..%d", top, bottom)
	}
	if got := Reveal(0, 5, 19, 20, 6); got != 14 {
		t.Fatalf("offset = %d, want the last page", got)
	}
	if got := Reveal(3, 1, 2, 5, 9); got != 0 {
		t.Fatalf("a window taller than its rows scrolled to %d", got)
	}
}
