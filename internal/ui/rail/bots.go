package rail

import (
	"strconv"
	"strings"

	"github.com/YoanWai/agent-manager/internal/ui/presentation"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// BotProfile is the copied part of a stored Bot Profile the rail lists.
type BotProfile struct {
	ID    string
	Name  string
	Emoji string
	// Color is an index into the theme's swatches.
	Color int
	Tool  string
}

// NoBotProfiles is what the empty Bots section says under its header.
const NoBotProfiles = "none yet · agent-manager bots"

// BotsFold is the Bots section's fold, saved the way RunsFold is.
var BotsFold = HostFold{Path: "bots"}

func (r treeRow) isBots() bool {
	return r.kind == BotsRow || r.kind == BotProfileRow || r.bot.ID != ""
}

// listedBot is a Profile the search and the filter let through, with the
// live Bots they let through under it.
type listedBot struct {
	profile BotProfile
	bots    []Session
}

// botProfileIDs are the stored Profiles; a session naming one is a Bot and
// lists under it rather than in the tree.
func (m Model) botProfileIDs() map[string]bool {
	ids := make(map[string]bool, len(m.snapshot.Bots))
	for _, profile := range m.snapshot.Bots {
		ids[profile.ID] = true
	}
	return ids
}

// treeSessions are the local sessions the tree lists. The archived view has
// no Bots section, so an archived Bot lists there like any session; so does
// a Bot whose Profile was deleted.
func (m Model) treeSessions() []Session {
	if m.showArchived || len(m.snapshot.Bots) == 0 {
		return m.snapshot.Sessions
	}
	profiles := m.botProfileIDs()
	sessions := make([]Session, 0, len(m.snapshot.Sessions))
	for _, session := range m.snapshot.Sessions {
		if !profiles[session.BotProfile] {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// listedBots is every Profile the search and the filter let through. A
// Profile matching the search keeps all its live Bots; otherwise it lists
// for the Bots that match. The attention filter keeps the Profiles with a
// Bot that needs the owner.
func (m Model) listedBots() []listedBot {
	query := strings.ToLower(strings.TrimSpace(m.search))
	live := map[string][]Session{}
	for _, session := range m.eligibleSessions("", m.snapshot.Sessions, m.snapshot.ArchivedGroups) {
		if session.BotProfile != "" {
			live[session.BotProfile] = append(live[session.BotProfile], session)
		}
	}
	var listed []listedBot
	for _, profile := range m.snapshot.Bots {
		bots := live[profile.ID]
		if query != "" && !strings.Contains(strings.ToLower(profile.Name+" "+profile.Tool), query) {
			var matched []Session
			for _, bot := range bots {
				if matchesSearch(bot, query) {
					matched = append(matched, bot)
				}
			}
			if len(matched) == 0 {
				continue
			}
			bots = matched
		}
		if m.filter == statusFilterAttention && len(bots) == 0 {
			continue
		}
		listed = append(listed, listedBot{profile: profile, bots: bots})
	}
	return listed
}

// botRows is the Bots section: a header, one row per listed Profile and,
// under an unfolded one, its live Bots. With no Profile stored the header
// stays, saying how to make one; the archived view and a search or filter
// that keeps nothing have no section.
func (m Model) botRows() []treeRow {
	if m.showArchived {
		return nil
	}
	listed := m.listedBots()
	if len(listed) == 0 && m.pruned() {
		return nil
	}
	rows := []treeRow{{kind: BotsRow}}
	if m.honorFolds() && m.hostFolds[BotsFold] {
		return rows
	}
	for _, entry := range listed {
		rows = append(rows, treeRow{kind: BotProfileRow, depth: 1, bot: entry.profile})
		if m.honorFolds() && !m.openProfiles[entry.profile.ID] {
			continue
		}
		for _, bot := range entry.bots {
			rows = append(rows, treeRow{kind: SessionRow, depth: 2, sess: bot, bot: entry.profile})
		}
	}
	return rows
}

// botsHinted reports whether the Bots header carries the line saying how to
// make a Profile.
func (m Model) botsHinted() bool {
	return len(m.snapshot.Bots) == 0 && !m.hostFolds[BotsFold]
}

// liveBots are the listed Bots under a Profile, folded or not.
func (m Model) liveBots(id string) []Session {
	for _, entry := range m.listedBots() {
		if entry.profile.ID == id {
			return entry.bots
		}
	}
	return nil
}

func botsHeaderMenuItems(folded bool) []menuItem {
	fold := menuItem{label: "Fold", action: ToggleFold}
	if folded {
		fold.label = "Unfold"
	}
	return []menuItem{fold}
}

func botProfileMenuItems(folded bool) []menuItem {
	fold := menuItem{label: "Hide Bots", action: ToggleFold}
	if folded {
		fold.label = "Show Bots"
	}
	return menuSections([]menuItem{fold}, []menuItem{{label: "Spawn a Bot", action: SpawnBot}})
}

// botProfileIntent is a Bot Profile action, which only a Profile row takes;
// anywhere else the key does nothing.
func (m *Model) botProfileIntent(kind ActionKind) Decision {
	if row, ok := m.row(); !ok || row.kind != BotProfileRow {
		return Decision{Consumed: true}
	}
	return m.intent(kind)
}

func (r renderer) renderBotsHeader(selected bool, width int, pad, background string) renderedTreeRow {
	style := r.palette.subtle.Bold(true)
	if selected {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color(r.ctx.Theme.Bright)).Bold(true)
	}
	marker := r.glyphs().GroupOpenGlyph
	if r.model.rowFolded(treeRow{kind: BotsRow}) {
		marker = r.glyphs().GroupClosedGlyph
	}
	count := "(" + strconv.Itoa(len(r.model.listedBots())) + ")"
	head := pad + r.palette.subtle.Render(marker) + " " + style.Render("Bots") + " " + r.palette.subtle.Render(count)
	text := paint(head, width, background)
	if r.model.botsHinted() {
		full := width
		if !r.ctx.MouseDisabled {
			full += menuButtonWidth
		}
		indent := pad + "  "
		hint := ansi.Truncate(NoBotProfiles, max(full-ansi.StringWidth(indent)-1, 0), "…")
		text += "\n" + paint(indent+r.palette.muted.Render(hint), full, background)
	}
	return renderedTreeRow{text: text, handleX: -1, foldX: ansi.StringWidth(pad)}
}

// swatch is the Profile's colour in this theme. An index past the swatches
// a theme hands out wraps, so a stored value always paints.
func (r renderer) swatch(index int) lipgloss.Color {
	swatches := r.ctx.Theme.Swatches()
	index %= len(swatches)
	if index < 0 {
		index += len(swatches)
	}
	return lipgloss.Color(swatches[index])
}

// renderBotProfile is the fold mark, the emoji and the name in the Profile's
// colour, and on the right its tool and how many Bots are live.
func (r renderer) renderBotProfile(row treeRow, selected bool, width int, pad, guides, background string) renderedTreeRow {
	profile := row.bot
	marker := r.glyphs().GroupOpenGlyph
	if r.model.rowFolded(row) {
		marker = r.glyphs().GroupClosedGlyph
	}
	nameStyle := lipgloss.NewStyle().Foreground(r.swatch(profile.Color)).Bold(true)
	if selected {
		nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(r.ctx.Theme.Bright)).Bold(true)
	}
	lead := pad + guides + r.palette.subtle.Render(marker) + " "
	if emoji := presentation.EscapeControlsInline(profile.Emoji); emoji != "" {
		lead += lipgloss.NewStyle().Foreground(r.swatch(profile.Color)).Render(emoji) + " "
	}
	bots := r.model.liveBots(profile.ID)
	meta := r.palette.subtle.Render(presentation.EscapeControlsInline(profile.Tool))
	if len(bots) > 0 {
		meta = r.botStatusGlyphs(bots) + r.palette.subtle.Render(" · ") + meta
	}
	room := width - railGutter - ansi.StringWidth(lead) - ansi.StringWidth(meta) - 2
	name := presentation.EscapeControlsInline(profile.Name)
	if ansi.StringWidth(name) > room {
		name = ansi.Truncate(name, max(room, 0), "…")
	}
	head := lead + r.palette.highlight(name, r.model.search, nameStyle)
	return renderedTreeRow{text: paint(rowColumns(head, meta, width-railGutter), width, background), handleX: -1, foldX: ansi.StringWidth(pad + guides)}
}

// botStatusGlyphs counts a Profile's live Bots by status, the way a group
// row counts its agents.
func (r renderer) botStatusGlyphs(bots []Session) string {
	counts := map[string]int{}
	for _, bot := range bots {
		counts[bot.Status]++
	}
	var parts []string
	for _, state := range rollupStates {
		if counts[state] > 0 {
			parts = append(parts, lipgloss.NewStyle().Foreground(r.palette.statusColor(state)).
				Render(r.glyphs().StatusGlyph(state)+" "+strconv.Itoa(counts[state])))
		}
	}
	return strings.Join(parts, r.palette.subtle.Render("  "))
}

// nothingListed reports a rail with no session, Run, Automation or Profile
// to show: the root row and, at most, the empty Bots header over the
// rail's own empty state.
func (m Model) nothingListed() bool {
	if len(m.rows) == 0 || !m.rows[0].isRoot() {
		return false
	}
	for _, row := range m.rows[1:] {
		if row.kind != BotsRow || len(m.snapshot.Bots) > 0 {
			return false
		}
	}
	return true
}
