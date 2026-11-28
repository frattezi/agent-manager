package presentation

// ChromeSet keeps fixed-width decorations separate from ASCII row marks.
// Compatible retains the thin rules and full block verified in the font audit.
type ChromeSet struct {
	ChoiceLeftGlyph, ChoiceRightGlyph                                       string
	TopLeftGlyph, TopRightGlyph, BottomLeftGlyph, BottomRightGlyph          string
	RuleGlyph, DashedRuleGlyph, ScrollThumbGlyph, SectionGlyph, CursorGlyph string
	ReviewSelectionGlyph, QuickEdgeGlyph, ResizeGlyph                       string
	// EdgeGlyph and SideGlyph are a frame's edges between its corners, kept
	// apart from RuleGlyph so a Compatible frame stays ASCII end to end.
	EdgeGlyph, SideGlyph string
	// Marks are an answer's state as a form draws it: a choice of one and a
	// box of many, each on or off. A mode's pair shares a width.
	ChoiceOnMark, ChoiceOffMark, CheckOnMark, CheckOffMark string
	// CaretGlyph is where typing lands in a focused text field, one cell
	// wide in every mode.
	CaretGlyph string
}

func ChromeFor(mode GlyphMode) ChromeSet {
	if mode.Effective() == GlyphModeAdvanced {
		return ChromeSet{
			ChoiceLeftGlyph: "◂", ChoiceRightGlyph: "▸", TopLeftGlyph: "╭", TopRightGlyph: "╮", BottomLeftGlyph: "╰", BottomRightGlyph: "╯",
			RuleGlyph: "━", DashedRuleGlyph: "┄", ScrollThumbGlyph: "┃", SectionGlyph: "▍", CursorGlyph: "▏",
			ReviewSelectionGlyph: "▎", QuickEdgeGlyph: "▌", ResizeGlyph: "║",
			EdgeGlyph: "─", SideGlyph: "│",
			ChoiceOnMark: "(*)", ChoiceOffMark: "( )", CheckOnMark: "[x]", CheckOffMark: "[ ]",
			CaretGlyph: "▏",
		}
	}
	return ChromeSet{
		ChoiceLeftGlyph: "<", ChoiceRightGlyph: ">", TopLeftGlyph: "+", TopRightGlyph: "+", BottomLeftGlyph: "+", BottomRightGlyph: "+",
		RuleGlyph: "─", DashedRuleGlyph: "─", ScrollThumbGlyph: "█", SectionGlyph: "│", CursorGlyph: "│",
		ReviewSelectionGlyph: "│", QuickEdgeGlyph: "│", ResizeGlyph: "│",
		EdgeGlyph: "-", SideGlyph: "|",
		ChoiceOnMark: "(*)", ChoiceOffMark: "( )", CheckOnMark: "[x]", CheckOffMark: "[ ]",
		CaretGlyph: "|",
	}
}
