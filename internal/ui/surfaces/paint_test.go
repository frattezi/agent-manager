package surfaces

import (
	"fmt"
	"strings"
	"testing"

	"github.com/YoanWai/agent-manager/internal/ui/a2ui"
	uitheme "github.com/YoanWai/agent-manager/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// sgr is the parameters a true-colour sequence names hex with.
func sgr(hex string, background bool) string {
	r, g, b := uitheme.HexRGB(hex)
	layer := 38
	if background {
		layer = 48
	}
	return fmt.Sprintf("%d;2;%d;%d;%d", layer, r, g, b)
}

func trueColor(t *testing.T) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
}

func withTheme(t *testing.T, name string) {
	t.Helper()
	prev := uitheme.Current()
	uitheme.Apply(uitheme.Palettes[uitheme.Index(name)])
	t.Cleanup(func() { uitheme.Apply(prev) })
}

func render(s lipgloss.Style, text string) string { return s.Render(text) }

func TestEveryRoleHasAStyle(t *testing.T) {
	for role := a2ui.RoleBody; role <= a2ui.RoleGap; role++ {
		if _, ok := roleStyles[role]; !ok {
			t.Errorf("%s has no style", role)
		}
	}
}

func TestRolesTakeTheThemeRolesTheProductUses(t *testing.T) {
	trueColor(t)
	cases := []struct {
		role a2ui.Role
		want lipgloss.Style
	}{
		{a2ui.RoleTitle, uitheme.SectionStyle()},
		{a2ui.RoleMuted, uitheme.MutedStyle()},
		{a2ui.RoleAccent, uitheme.KeyStyle()},
		{a2ui.RoleBody, uitheme.ValueStyle()},
	}
	for _, c := range cases {
		if got := paint([]a2ui.Line{{{Text: "x", Role: c.role}}})[0]; got != render(c.want, "x") {
			t.Errorf("%s paints %q, want %q", c.role, got, render(c.want, "x"))
		}
	}
	heading := paint([]a2ui.Line{{{Text: "x", Role: a2ui.RoleHeading}}})[0]
	if heading == render(uitheme.ValueStyle(), "x") {
		t.Errorf("a heading paints as body text: %q", heading)
	}
}

// The focused control sits on the selected-row fill, and the cursor and
// what is chosen read apart from each other and from the rest of it.
func TestTheFocusedControlIsFilledAndItsCursorStandsOut(t *testing.T) {
	trueColor(t)
	fillSeq := sgr(uitheme.SelectedHex(), true)
	painted := func(role a2ui.Role, filled bool) string {
		return paint([]a2ui.Line{{{Text: "x", Role: role, Filled: filled}}})[0]
	}
	if got := painted(a2ui.RoleLabel, true); !strings.Contains(got, fillSeq) {
		t.Errorf("a focused label %q is not on the fill %q", got, fillSeq)
	}
	if got := painted(a2ui.RoleLabel, false); got == painted(a2ui.RoleLabel, true) {
		t.Errorf("a label paints the same filled and not: %q", got)
	}
	cursor, chosen, rest := painted(a2ui.RoleFocused, true), painted(a2ui.RoleSelected, true), painted(a2ui.RoleBody, true)
	if cursor == chosen || cursor == rest || chosen == rest {
		t.Errorf("cursor %q, chosen %q and rest %q are not all apart", cursor, chosen, rest)
	}
}

func TestTheCardRepaintsInATheme(t *testing.T) {
	trueColor(t)
	card, h := openRound(t)
	withTheme(t, "classic")
	card.View(h, "")
	before := h.body
	uitheme.Apply(uitheme.Palettes[uitheme.Index("nord")])
	card.View(h, "")
	if h.body == before {
		t.Fatal("the card kept the old theme")
	}
	if !strings.Contains(h.body, sgr(uitheme.Current().Accent, false)) {
		t.Errorf("the card does not carry the new accent:\n%q", h.body)
	}
}

// The card frame drops nothing the plain rendering lays out.
func TestPaintKeepsTheLayout(t *testing.T) {
	trueColor(t)
	lines, _ := mustOpen(t).render(60)
	for i, p := range paint(lines) {
		if got := ansi.Strip(p); got != lines[i].Text() {
			t.Errorf("line %d paints as %q, want %q", i, got, lines[i].Text())
		}
	}
}

func mustOpen(t *testing.T) *Card {
	card, _ := openRound(t)
	return card
}
