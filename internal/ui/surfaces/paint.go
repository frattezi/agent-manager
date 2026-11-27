package surfaces

import (
	"strings"

	"github.com/YoanWai/agent-manager/internal/ui/a2ui"
	uitheme "github.com/YoanWai/agent-manager/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// roleStyles is the one place a role the renderer emits becomes a style.
// Each entry reads the live theme when it is called, so a card repaints in
// a theme applied while it is open.
var roleStyles = map[a2ui.Role]func() lipgloss.Style{
	a2ui.RoleBody:    uitheme.ValueStyle,
	a2ui.RoleTitle:   uitheme.SectionStyle,
	a2ui.RoleHeading: func() lipgloss.Style { return lipgloss.NewStyle().Bold(true).Foreground(uitheme.ColorBright()) },
	a2ui.RoleMuted:   uitheme.MutedStyle,
	a2ui.RoleLabel:   func() lipgloss.Style { return lipgloss.NewStyle().Foreground(uitheme.ColorBright()) },
	a2ui.RoleValue:   uitheme.ValueStyle,
	a2ui.RoleAccent:  uitheme.KeyStyle,
	// The cursor and the caret are the one solid patch inside the filled
	// control, so they read apart from both the fill and what is chosen.
	a2ui.RoleFocused:  uitheme.SearchMatchStyle,
	a2ui.RoleSelected: uitheme.InboxBadgeStyle,
	a2ui.RoleRule:     uitheme.SubtleStyle,
	a2ui.RoleGap:      uitheme.MutedStyle,
}

// fill is what a segment of the focused control is drawn over, the band a
// selected row is filled with everywhere else. A style that brings its own
// background keeps it.
func fill(style lipgloss.Style) lipgloss.Style {
	if _, none := style.GetBackground().(lipgloss.NoColor); !none {
		return style
	}
	return style.Background(uitheme.SelectedRowStyle().GetBackground())
}

func styleFor(s a2ui.Segment) lipgloss.Style {
	style := lipgloss.NewStyle()
	if styled, ok := roleStyles[s.Role]; ok {
		style = styled()
	}
	if s.Filled {
		style = fill(style)
	}
	return style
}

// paint draws each line's segments in their styles.
func paint(lines []a2ui.Line) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		var b strings.Builder
		for _, s := range l {
			b.WriteString(styleFor(s).Render(s.Text))
		}
		out[i] = b.String()
	}
	return out
}
