package overview

import (
	"strconv"
	"strings"

	"github.com/YoanWai/agent-manager/internal/ui/presentation"
)

// BotProfile is a Bot Profile with its live Bots, as the Bots Overview draws it.
type BotProfile struct {
	Name  string
	Emoji string
	// Color is an index into the palette's swatches.
	Color int
	Tool  string
	Bots  []Bot
}

// Bot is one live Bot: its name and the status the poller last read for it.
type Bot struct {
	Name   string
	Status string
}

// DrawBots renders the Bots Overview at width: each Profile in its colour with
// its live Bots as status dots, and a Profile with no live Bot dimmed. Every
// line is at most width cells wide. It is live state, so there is no window.
func DrawBots(profiles []BotProfile, width int, mode presentation.GlyphMode, palette Palette) []string {
	p := painter{palette: palette}
	glyphs := presentation.GlyphsFor(mode)
	inner := max(width-2, 10)
	live := 0
	for _, profile := range profiles {
		live += len(profile.Bots)
	}
	lines := []string{fit(p.fg(palette.Dim).Render("Profiles ")+p.fg(palette.Bright).Bold(true).Render(strconv.Itoa(len(profiles)))+
		p.fg(palette.Dim).Render("  Live Bots ")+p.fg(palette.Bright).Bold(true).Render(strconv.Itoa(live)), inner), ""}
	for _, profile := range profiles {
		color := palette.Dim
		if n := len(palette.Swatches); n > 0 && len(profile.Bots) > 0 {
			color = palette.Swatches[((profile.Color%n)+n)%n]
		}
		head := p.fg(color).Bold(true).Render(strings.TrimSpace(presentation.EscapeControlsInline(profile.Emoji) + " " + presentation.EscapeControlsInline(profile.Name)))
		head += p.fg(palette.Dim).Render("  " + presentation.EscapeControlsInline(profile.Tool))
		if len(profile.Bots) == 0 {
			lines = append(lines, fit(head+p.fg(palette.Dim).Render("  no live Bots"), inner))
			continue
		}
		lines = append(lines, fit(head, inner))
		for _, bot := range profile.Bots {
			dot := p.fg(botColor(palette, bot.Status)).Render(glyphs.StatusGlyph(bot.Status))
			lines = append(lines, fit("  "+dot+" "+p.fg(palette.Text).Render(presentation.EscapeControlsInline(bot.Name))+p.fg(palette.Dim).Render("  "+bot.Status), inner))
		}
	}
	return lines
}

func botColor(p Palette, status string) string {
	switch status {
	case "working", "starting":
		return p.Working
	case "waiting":
		return p.Waiting
	case "finished":
		return p.Finished
	case "errored", "dead":
		return p.Errored
	}
	return p.Idle
}
