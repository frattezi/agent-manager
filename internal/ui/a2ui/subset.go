// Package a2ui reads an Agent Surface, an A2UI document an agent wrote, and
// draws it as terminal text. It is the format half of Agent Surfaces: a
// document goes in with a width and text comes out, with no store, tmux, Git,
// clock or file behind it. The text comes as segments that each carry a Role,
// what the text is; how a role looks is the caller's, so the package reads no
// theme and no lipgloss.
//
// A document is a JSON array of A2UI messages (createSurface,
// updateComponents, updateDataModel) for one surface, in the order a stream
// would carry them. Parsing is strict about the version each message claims
// and lenient about the rest: unknown fields and unknown message kinds are
// ignored, and a component outside the subset is kept as a labelled gap.
package a2ui

import "sort"

// Version is the A2UI version this build reads.
const Version = "v0.9"

// component is how one type in the subset is read from its properties and
// drawn at a width. draw gets its children's drawing from the pen, so the
// subset can name every component without referring back to itself.
type component struct {
	read func(p props) (Node, []childRef, error)
	draw func(p pen, n Node, width int) []line
}

// subset is every component this build draws, and the only place one is
// added: a type missing here parses as a gap. Text, Card, Column, Row, List
// and Divider lay out; Button, TextField, CheckBox and ChoicePicker ask.
var subset = map[string]component{
	"Text":         {readText, drawText},
	"Card":         {readCard, drawCard},
	"Column":       {readColumn, drawColumn},
	"Row":          {readRow, drawRow},
	"List":         {readList, drawList},
	"Divider":      {readDivider, drawDivider},
	"Button":       {readButton, drawButton},
	"TextField":    {readTextField, drawTextField},
	"CheckBox":     {readCheckBox, drawCheckBox},
	"ChoicePicker": {readChoicePicker, drawChoicePicker},
}

// Subset names the components this build draws, sorted.
func Subset() []string {
	names := make([]string, 0, len(subset))
	for name := range subset {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
