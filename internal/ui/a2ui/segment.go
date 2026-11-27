package a2ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Role says what a piece of drawn text is, never how it looks: the caller
// owns the table from a role to a style, and the plain rendering ignores it.
type Role int

const (
	RoleBody Role = iota
	RoleTitle
	RoleHeading
	RoleMuted
	RoleLabel
	RoleValue
	RoleAccent
	RoleFocused
	RoleSelected
	RoleRule
	RoleGap
)

var roleNames = [...]string{"body", "title", "heading", "muted", "label", "value", "accent", "focused", "selected", "rule", "gap"}

func (r Role) String() string {
	if r < 0 || int(r) >= len(roleNames) {
		return "unknown"
	}
	return roleNames[r]
}

// variantRoles is the one place an A2UI variant decides a role. A variant
// missing here keeps the component's ordinary role.
var variantRoles = map[string]map[string]Role{
	"Text": {
		"h1":      RoleTitle,
		"h2":      RoleHeading,
		"h3":      RoleHeading,
		"caption": RoleMuted,
	},
	"Button": {
		"primary": RoleAccent,
	},
}

func variantRole(n Node, fallback Role) Role {
	if role, ok := variantRoles[n.Type][n.Variant]; ok {
		return role
	}
	return fallback
}

// Segment is a run of text on one line that carries one role. Filled says
// it belongs to the control the focus is on, which is drawn as one block.
type Segment struct {
	Text   string
	Role   Role
	Filled bool
}

// Line is one drawn row, its segments in reading order.
type Line []Segment

// Text is the line with its roles dropped.
func (l Line) Text() string {
	var b strings.Builder
	for _, s := range l {
		b.WriteString(s.Text)
	}
	return b.String()
}

// Width is the line's width in cells.
func (l Line) Width() int {
	width := 0
	for _, s := range l {
		width += ansi.StringWidth(s.Text)
	}
	return width
}

// Texts drops the roles of every line.
func Texts(lines []Line) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.Text()
	}
	return out
}

// add returns the line with text appended under role, joined to the last
// segment when the role is the same, so a line holds one segment per change
// of role. It never writes into l, so a shared prefix stays as it was.
func (l Line) add(role Role, text string) Line {
	if text == "" {
		return l
	}
	out := make(Line, len(l), len(l)+1)
	copy(out, l)
	if n := len(out); n > 0 && out[n-1].Role == role && !out[n-1].Filled {
		out[n-1].Text += text
		return out
	}
	return append(out, Segment{Text: text, Role: role})
}

func (l Line) join(other Line) Line {
	for _, s := range other {
		if s.Filled {
			l = append(l[:len(l):len(l)], s)
			continue
		}
		l = l.add(s.Role, s.Text)
	}
	return l
}

// filled marks every segment of the line as part of the focused control.
func (l Line) filled() Line {
	out := append(Line(nil), l...)
	for i := range out {
		out[i].Filled = true
	}
	return out
}

func (l Line) padRight(width int) Line {
	return l.add(RoleBody, strings.Repeat(" ", max(width-l.Width(), 0)))
}

func (l Line) trimRight() Line {
	out := append(Line(nil), l...)
	for len(out) > 0 {
		last := &out[len(out)-1]
		if last.Filled {
			break
		}
		last.Text = strings.TrimRight(last.Text, " ")
		if last.Text != "" {
			break
		}
		out = out[:len(out)-1]
	}
	return out
}

func seg(role Role, text string) Line { return Line(nil).add(role, text) }
