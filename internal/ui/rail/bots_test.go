package rail

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/YoanWai/agent-manager/internal/keybind"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func botsSnapshot() Snapshot {
	return Snapshot{
		Sessions: []Session{
			{ID: "work", Name: "work", Tool: "claude", Status: "idle", CreatedAt: time.Now()},
			{ID: "watch-runs", Name: "watch runs", Tool: "claude", Status: "working", BotProfile: "op", CreatedAt: time.Now()},
			{ID: "watch-host", Name: "watch host", Tool: "claude", Status: "waiting", BotProfile: "op", CreatedAt: time.Now()},
			{ID: "stray", Name: "stray", Tool: "codex", Status: "idle", BotProfile: "deleted", CreatedAt: time.Now()},
		},
		Bots: []BotProfile{
			{ID: "op", Name: "operator", Emoji: "🛠", Color: 2, Tool: "claude"},
			{ID: "rev", Name: "reviewer", Emoji: "🔎", Tool: "codex"},
		},
	}
}

func frameText(frame Frame) string {
	var text strings.Builder
	for _, line := range frame.Lines {
		text.WriteString(ansi.Strip(line.Text) + "\n")
	}
	return text.String()
}

func pressOn(model *Model, selection Selection, key string) Decision {
	model.Focus(selection)
	return model.Key(keyFor(key), Frame{}, KeyContext{ListKeys: keybind.DefaultList()})
}

func keyFor(key string) tea.KeyMsg {
	if key == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

var operator = Selection{Kind: BotProfileRow, Profile: "op"}

func TestBotsListBelowAutomationsWithTheirBotsOutOfTheTree(t *testing.T) {
	model := New(nil)
	snapshot := botsSnapshot()
	snapshot.Automations = []Automation{{ID: "a", Name: "a", Enabled: true}}
	model.Reconcile(snapshot)
	want := []string{"g:", "s:work", "s:stray", "automations", " a:a", "bots", " p:op", " p:rev"}
	if got := rowKeys(model); !slices.Equal(got, want) {
		t.Fatalf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestAProfileRowCarriesItsEmojiNameToolAndLiveBots(t *testing.T) {
	model := New(nil)
	model.Reconcile(botsSnapshot())
	text := frameText(model.Render(hostsContext(56, 14), Frame{}))
	for _, want := range []string{"Bots (2)", "🛠 operator", "claude", "🔎 reviewer", "codex"} {
		if !strings.Contains(text, want) {
			t.Fatalf("frame lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "watch runs") || strings.Contains(text, NoBotProfiles) {
		t.Fatalf("a folded Profile lists its Bots, or a full area says it is empty:\n%s", text)
	}
}

func TestAnUnfoldedProfileListsItsLiveBotsAsSessionRows(t *testing.T) {
	model := New(nil)
	model.Reconcile(botsSnapshot())
	if decision := pressOn(&model, operator, "enter"); len(decision.Mutations) != 1 || decision.Intent.Kind != NoAction {
		t.Fatalf("enter on a Profile: %+v", decision)
	}
	want := []string{"g:", "s:work", "s:stray", "bots", " p:op", "  s:watch-runs", "  s:watch-host", " p:rev"}
	if got := rowKeys(model); !slices.Equal(got, want) {
		t.Fatalf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	text := frameText(model.Render(hostsContext(64, 14), Frame{}))
	for _, want := range []string{"watch runs", "working", "watch host", "waiting"} {
		if !strings.Contains(text, want) {
			t.Fatalf("frame lacks %q:\n%s", want, text)
		}
	}
	bot := Selection{Kind: SessionRow, SessionID: "watch-host"}
	plain := Selection{Kind: SessionRow, SessionID: "work"}
	if got, want := pressOn(&model, bot, "a").Intent, pressOn(&model, plain, "a").Intent; got.Kind != want.Kind || got.Target != bot {
		t.Fatalf("a on a Bot = %+v; a session takes %+v", got, want)
	}
	if !slices.Equal(menuLabels(model, bot), menuLabels(model, plain)) {
		t.Fatalf("a Bot's menu %v differs from a session's", menuLabels(model, bot))
	}
	if decision := pressOn(&model, bot, "K"); decision.Error == "" || len(decision.Mutations) != 0 {
		t.Fatalf("reordering a Bot: %+v", decision)
	}
}

func TestAnEmptyBotsAreaSaysHowToCreateAProfile(t *testing.T) {
	model := New(nil)
	model.Reconcile(Snapshot{Sessions: []Session{{ID: "work", Name: "work", Status: "idle"}}})
	text := frameText(model.Render(hostsContext(64, 10), Frame{}))
	if !strings.Contains(text, "Bots (0)") || !strings.Contains(text, NoBotProfiles) {
		t.Fatalf("empty area:\n%s", text)
	}

	empty := New(nil)
	empty.Reconcile(Snapshot{})
	text = frameText(empty.Render(hostsContext(64, 14), Frame{}))
	if !strings.Contains(text, "no sessions yet") || !strings.Contains(text, NoBotProfiles) {
		t.Fatalf("an empty rail loses its own empty state or the Bots hint:\n%s", text)
	}
	if selected, _ := empty.Selected(); selected.Kind != GroupRow {
		t.Fatalf("an empty rail starts on %+v, not the root", selected)
	}
}

func TestEveryBotsKeyHasARowMenuEntry(t *testing.T) {
	model := New(nil)
	model.Reconcile(botsSnapshot())
	if got := menuLabels(model, operator); !slices.Equal(got, []string{"Show Bots"}) {
		t.Fatalf("folded Profile menu = %v", got)
	}
	if got := menuLabels(model, Selection{Kind: BotsRow}); !slices.Equal(got, []string{"Fold"}) {
		t.Fatalf("Bots header menu = %v", got)
	}
	model.openMenu(operator, 0, 0, false)
	model.menuKey(tea.KeyMsg{Type: tea.KeyEnter}, KeyContext{ListKeys: keybind.DefaultList()})
	if !slices.Contains(rowKeys(model), "  s:watch-runs") {
		t.Fatalf("Show Bots left the Profile folded: %v", rowKeys(model))
	}
	if got := menuLabels(model, operator); !slices.Equal(got, []string{"Hide Bots"}) {
		t.Fatalf("unfolded Profile menu = %v", got)
	}
	pressOn(&model, Selection{Kind: BotsRow}, "enter")
	if got := rowKeys(model); slices.Contains(got, " p:op") {
		t.Fatalf("enter left the Bots header open: %v", got)
	}
	if got := menuLabels(model, Selection{Kind: BotsRow}); !slices.Equal(got, []string{"Unfold"}) {
		t.Fatalf("folded Bots header menu = %v", got)
	}
}

func TestClickingABotsFoldMarkFolds(t *testing.T) {
	model := New(nil)
	model.Reconcile(botsSnapshot())
	click := func(selection Selection) {
		t.Helper()
		frame := model.Render(hostsContext(56, 14), Frame{})
		x, ok := frame.Folds[selection.key()]
		if !ok {
			t.Fatalf("no fold mark for %s in %v", selection.key(), frame.Folds)
		}
		for y, line := range frame.Lines {
			if line.HitOK && line.Hit == selection {
				model.Mouse(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}, frame, MouseContext{FullWidth: true, Now: time.Now()})
				return
			}
		}
		t.Fatalf("no line for %s", selection.key())
	}
	click(operator)
	if !slices.Contains(rowKeys(model), "  s:watch-runs") {
		t.Fatalf("a click on the Profile's mark left it folded: %v", rowKeys(model))
	}
	click(Selection{Kind: BotsRow})
	if slices.Contains(rowKeys(model), " p:op") {
		t.Fatalf("a click on the header's mark left it open: %v", rowKeys(model))
	}
}

func TestSearchFindsProfilesByNameAndTool(t *testing.T) {
	model := New(nil)
	model.Reconcile(botsSnapshot())
	profiles := func() []string {
		var out []string
		for _, row := range model.Rows() {
			if row.Kind == BotProfileRow {
				out = append(out, row.Name)
			}
		}
		return out
	}
	model.SetSearch("review", false)
	if got := profiles(); !slices.Equal(got, []string{"reviewer"}) {
		t.Fatalf("search by name = %v", got)
	}
	model.SetSearch("codex", false)
	if got := profiles(); !slices.Equal(got, []string{"reviewer"}) {
		t.Fatalf("search by tool = %v", got)
	}
	model.SetSearch("watch host", false)
	if got := rowKeys(model); !slices.Contains(got, " p:op") || !slices.Contains(got, "  s:watch-host") || slices.Contains(got, "  s:watch-runs") {
		t.Fatalf("a search matching a Bot = %v", got)
	}
	model.SetSearch("nothing like it", false)
	if slices.Contains(rowKeys(model), "bots") {
		t.Fatalf("a search matching nothing keeps the Bots section: %v", rowKeys(model))
	}
}

func TestTheArchivedViewHasNoBotsSection(t *testing.T) {
	model := New(nil)
	snapshot := botsSnapshot()
	snapshot.Sessions = append(snapshot.Sessions, Session{ID: "old-bot", Name: "old bot", Status: "dead", Archived: true, BotProfile: "op"})
	model.Reconcile(snapshot)
	model.SetArchived(true)
	got := rowKeys(model)
	if slices.Contains(got, "bots") || slices.Contains(got, " p:op") {
		t.Fatalf("archived view lists Bots: %v", got)
	}
	if !slices.Contains(got, "s:old-bot") {
		t.Fatalf("an archived Bot is missing from the archived view: %v", got)
	}
}

func TestAttentionKeepsTheProfilesWithABotWaiting(t *testing.T) {
	model := New(nil)
	model.Reconcile(botsSnapshot())
	model.SetFilteringAttention(true)
	got := rowKeys(model)
	if !slices.Contains(got, " p:op") || !slices.Contains(got, "  s:watch-host") || slices.Contains(got, " p:rev") {
		t.Fatalf("attention = %v", got)
	}
}
