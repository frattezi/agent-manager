package presentation

import (
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestChromeKeepsOneCellAndCompatibleFontRepertoire(t *testing.T) {
	for _, mode := range []GlyphMode{GlyphModeBasic, GlyphModeAdvanced, ""} {
		chrome := reflect.ValueOf(ChromeFor(mode))
		for i := 0; i < chrome.NumField(); i++ {
			mark := chrome.Field(i).String()
			if strings.HasSuffix(chrome.Type().Field(i).Name, "Mark") {
				continue
			}
			if ansi.StringWidth(mark) != 1 {
				t.Errorf("%s %s occupies %d cells", mode, chrome.Type().Field(i).Name, ansi.StringWidth(mark))
			}
			if mode == GlyphModeBasic {
				for _, r := range mark {
					if (r < '!' || r > '~') && !strings.ContainsRune("─│█", r) {
						t.Errorf("unverified compatible decoration %q", mark)
					}
				}
			}
		}
	}
	if ChromeFor("") != ChromeFor(DefaultGlyphMode) {
		t.Fatal("unset chrome did not follow the default")
	}
}

func TestMarksShareAWidthAndCompatibleMarksAndEdgesAreASCII(t *testing.T) {
	for _, mode := range []GlyphMode{GlyphModeBasic, GlyphModeAdvanced} {
		c := ChromeFor(mode)
		for _, pair := range [][2]string{{c.ChoiceOnMark, c.ChoiceOffMark}, {c.CheckOnMark, c.CheckOffMark}} {
			if ansi.StringWidth(pair[0]) == 0 || ansi.StringWidth(pair[0]) != ansi.StringWidth(pair[1]) {
				t.Errorf("%s marks %q and %q differ in width", mode, pair[0], pair[1])
			}
		}
	}
	c := ChromeFor(GlyphModeBasic)
	frame := c.TopLeftGlyph + c.TopRightGlyph + c.BottomLeftGlyph + c.BottomRightGlyph + c.EdgeGlyph + c.SideGlyph +
		c.ChoiceOnMark + c.ChoiceOffMark + c.CheckOnMark + c.CheckOffMark
	for _, r := range frame {
		if r < ' ' || r > '~' {
			t.Errorf("compatible frame or mark carries %q", r)
		}
	}
}
