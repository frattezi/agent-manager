package rail

import (
	"slices"
	"testing"

	"github.com/YoanWai/agent-manager/internal/keybind"
	tea "github.com/charmbracelet/bubbletea"
)

func TestGroupMenuReturnsTypedGroupIntent(t *testing.T) {
	model := New(nil)
	model.Reconcile(Snapshot{
		Groups:   []string{"work"},
		Sessions: []Session{{ID: "agent", Name: "agent", Group: "work"}},
	})
	model.Focus(Selection{Kind: GroupRow})

	decision := model.openMenu(Selection{Kind: GroupRow, Group: "work"}, 4, 5, false)
	if !decision.SelectionChanged || decision.SelectionEffect != SelectionEffectPreview {
		t.Fatalf("open decision = %+v, want preview selection", decision)
	}

	rename := -1
	for index, item := range model.menu.items {
		if item.action == Attach {
			t.Fatal("group menu offered attach")
		}
		if item.action == RenameAction {
			rename = index
		}
	}
	if rename < 0 {
		t.Fatal("group menu omitted rename")
	}

	decision = model.runMenuItem(rename)
	if decision.Intent.Kind != RenameAction || decision.Intent.Target != (Selection{Kind: GroupRow, Group: "work"}) {
		t.Fatalf("rename decision = %+v", decision)
	}
}

func TestArchivedToggleSwapsRowsAtOnce(t *testing.T) {
	model := New(nil)
	model.Reconcile(Snapshot{
		Sessions: []Session{{ID: "active", Name: "active"}, {ID: "old", Name: "old", Archived: true}},
		Hosts: []Host{{Name: "box", State: HostOnline, Sessions: []Session{
			{ID: "remote-active", Name: "remote-active"}, {ID: "remote-old", Name: "remote-old", Archived: true},
		}}},
	})
	model.Focus(Selection{Kind: SessionRow, SessionID: "active"})

	for _, step := range []struct {
		archived bool
		want     []string
	}{
		{archived: true, want: []string{"old", "remote-old"}},
		{archived: false, want: []string{"active", "remote-active"}},
	} {
		decision := model.action(keybind.Archived, KeyContext{})
		if !decision.Refresh || model.ShowArchived() != step.archived {
			t.Fatalf("archive toggle = %+v showArchived=%v", decision, model.ShowArchived())
		}
		var sessions []string
		for _, row := range model.Rows() {
			if row.Kind == SessionRow {
				sessions = append(sessions, row.SessionID)
			}
		}
		if !slices.Equal(sessions, step.want) {
			t.Fatalf("showArchived=%v rows = %v, want %v", step.archived, sessions, step.want)
		}
		if current, _ := model.Selected(); decision.Selection != current || decision.SelectionEffect != SelectionEffectFilter {
			t.Fatalf("showArchived=%v decision = %+v, selected %+v", step.archived, decision, current)
		}
	}
}

func TestPendingEndMenuOffersCancelOnLiveAndDeadRows(t *testing.T) {
	for _, state := range []string{"idle", "dead"} {
		model := New(nil)
		model.Reconcile(Snapshot{Sessions: []Session{{ID: "agent", Name: "agent", Status: state, AfterTurn: "kill", AfterTurnGlyph: "■"}}})
		selection := Selection{Kind: SessionRow, SessionID: "agent"}
		model.Focus(selection)
		model.openMenu(selection, 0, 0, false)
		cancel := -1
		for index, item := range model.menu.items {
			if item.label == "Cancel kill" && item.action == CancelEnd {
				cancel = index
			}
		}
		if cancel < 0 {
			t.Fatalf("%s row menu has no Cancel kill: %+v", state, model.menu.items)
		}
		if decision := model.runMenuItem(cancel); decision.Intent.Kind != CancelEnd || decision.Intent.Target != selection {
			t.Fatalf("%s cancel decision = %+v", state, decision)
		}
	}
}

func TestCancelEndKeyReturnsTypedIntent(t *testing.T) {
	model := New(nil)
	model.Reconcile(Snapshot{Sessions: []Session{{ID: "agent", Name: "agent", AfterTurn: "archive"}}})
	model.Focus(Selection{Kind: SessionRow, SessionID: "agent"})
	if decision := model.action(keybind.CancelEnd, KeyContext{}); decision.Intent.Kind != CancelEnd {
		t.Fatalf("cancel key decision = %+v", decision)
	}
}

func TestGroupAndRootMenusOfferQuickPromptMode(t *testing.T) {
	model := New(nil)
	model.Reconcile(Snapshot{Groups: []string{"work"}})
	for _, selection := range []Selection{{Kind: GroupRow, Group: "work"}, {Kind: GroupRow}} {
		model.openMenu(selection, 0, 0, false)
		if len(model.menu.items) == 0 || model.menu.items[0].label != "Quick prompt mode" || model.menu.items[0].action != Prompt {
			t.Fatalf("menu for %+v = %+v, want quick prompt mode first", selection, model.menu.items)
		}
	}
}

func TestArchivedRowsOfferRestoreWithoutRelaunch(t *testing.T) {
	model := New(nil)
	model.SetArchived(true)
	model.Reconcile(Snapshot{Groups: []string{"work"}, Sessions: []Session{{ID: "agent", Name: "agent", Group: "work", Archived: true, Status: "dead"}}})
	for _, selection := range []Selection{{Kind: GroupRow, Group: "work"}, {Kind: SessionRow, SessionID: "agent", Group: "work"}} {
		if !model.Focus(selection) {
			t.Fatalf("missing archived selection: %+v", selection)
		}
		for _, action := range []string{keybind.Revive, keybind.ReviveAll, keybind.Restart, keybind.Kill, keybind.KillAll} {
			if decision := model.action(action, KeyContext{}); !decision.Consumed || decision.Intent.Kind != NoAction {
				t.Fatalf("archived %v %s = %+v", selection, action, decision)
			}
		}
		model.openMenu(selection, 0, 0, false)
		restore := -1
		for index, item := range model.menu.items {
			if item.action == Revive || item.action == ReviveAll || item.action == Restart {
				t.Fatalf("archived menu offers relaunch: %+v", model.menu.items)
			}
			if item.action == Restore {
				restore = index
			}
		}
		if restore < 0 {
			t.Fatalf("archived menu missing restore: %+v", model.menu.items)
		}
		if decision := model.runMenuItem(restore); decision.Intent.Kind != Restore {
			t.Fatalf("restore = %+v", decision)
		}
		legend := model.RowLegend(LegendContext{ListKeys: keybind.DefaultList(), Row: selection, RowOK: true})
		for _, pair := range legend.Pairs {
			if pair[1] == "revive" || pair[1] == "restart" {
				t.Fatalf("archived legend offers relaunch: %+v", legend)
			}
		}
	}
}

func TestArchivedRowsKeepManagementKeysMenusAndFooter(t *testing.T) {
	model := New(nil)
	model.SetArchived(true)
	model.Reconcile(Snapshot{Groups: []string{"work"}, Sessions: []Session{{ID: "agent", Name: "agent", Group: "work", Archived: true, Status: "dead"}}})
	keys := keybind.DefaultList()
	for _, selection := range []Selection{{Kind: GroupRow, Group: "work"}, {Kind: SessionRow, SessionID: "agent", Group: "work"}} {
		if !model.Focus(selection) {
			t.Fatalf("missing archived selection: %+v", selection)
		}
		legend := model.RowLegend(LegendContext{ListKeys: keys, Row: selection, RowOK: true})
		for _, action := range []struct {
			key   string
			kind  ActionKind
			label string
		}{
			{keybind.Editor, OpenEditor, "editor"},
			{keybind.Rename, RenameAction, "rename"},
			{keybind.Move, MoveToGroup, "move"},
			{keybind.Restore, Restore, "restore"},
		} {
			if decision := model.Key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keys.Binding(action.key).Label())}, Frame{}, KeyContext{ListKeys: keys}); decision.Intent.Kind != action.kind || decision.Intent.Target != selection {
				t.Fatalf("archived %+v key %s = %+v", selection, action.key, decision)
			}
			model.openMenu(selection, 0, 0, false)
			index := -1
			for i, item := range model.menu.items {
				if item.action == action.kind {
					index = i
				}
			}
			if index < 0 {
				t.Fatalf("archived %+v menu missing %s: %+v", selection, action.key, model.menu.items)
			}
			if decision := model.runMenuItem(index); decision.Intent.Kind != action.kind || decision.Intent.Target != selection {
				t.Fatalf("archived %+v menu %s = %+v", selection, action.key, decision)
			}
			found := false
			for _, pair := range legend.Pairs {
				if pair[1] == action.label && pair[0] == keys.Binding(action.key).Glyph("/") {
					found = true
				}
			}
			if !found {
				t.Fatalf("archived %+v footer missing %s: %+v", selection, action.key, legend.Pairs)
			}
		}
	}
}

func TestArchivedCreationActionsKeepNewSessionAvailable(t *testing.T) {
	model := New(nil)
	model.SetArchived(true)
	model.Reconcile(Snapshot{Sessions: []Session{{ID: "archived", Name: "archived", Archived: true}}})
	model.Focus(Selection{Kind: SessionRow, SessionID: "archived"})
	for _, action := range []string{keybind.Fork, keybind.Terminal, keybind.Prompt} {
		if decision := model.action(action, KeyContext{}); decision.Intent.Kind != NoAction {
			t.Errorf("archived %s admitted intent %+v", action, decision.Intent)
		}
	}
	if decision := model.action(keybind.NewSession, KeyContext{}); decision.Intent.Kind != NewSession {
		t.Fatal("new session must remain available")
	}
}

func TestRootMenuOffersOnlyApplicableActions(t *testing.T) {
	for _, archived := range []bool{false, true} {
		model := New(nil)
		model.SetArchived(archived)
		model.Reconcile(Snapshot{Sessions: []Session{{ID: "live", Name: "live", Status: "working", Archived: archived}, {ID: "dead", Name: "dead", Status: "dead", Archived: archived}}})
		model.openMenu(Selection{Kind: GroupRow}, 0, 0, false)
		offered := map[ActionKind]bool{}
		for _, item := range model.menu.items {
			offered[item.action] = true
		}
		if !offered[NewSession] || !offered[OpenEditor] {
			t.Fatal("root menu omitted valid global creation/editor action")
		}
		if archived {
			for _, action := range []ActionKind{NewTerminal, Prompt, Fork, Kill, Revive} {
				if offered[action] {
					t.Errorf("archived root menu offers %v", action)
				}
			}
		} else if !offered[Kill] || !offered[Revive] {
			t.Fatal("active root menu omitted valid lifecycle actions")
		}
	}
}

func TestArchivedRemoteMenusMatchContextualCreationKeys(t *testing.T) {
	for _, selection := range []Selection{{Kind: ConnectionRow, Host: "box"}, {Kind: GroupRow, Host: "box", Group: "ops"}, {Kind: SessionRow, Host: "box", SessionID: "deep", Group: "ops"}} {
		model := New(nil)
		snapshot := hostsSnapshot(HostOnline)
		for i := range snapshot.Hosts[0].Sessions {
			snapshot.Hosts[0].Sessions[i].Archived = true
		}
		model.SetArchived(true)
		model.Reconcile(snapshot)
		if !model.Focus(selection) {
			t.Fatalf("remote target not visible: %+v", selection)
		}
		model.openMenu(selection, 0, 0, false)
		offered := map[ActionKind]bool{}
		for _, item := range model.menu.items {
			offered[item.action] = true
		}
		if offered[NewTerminal] || offered[Prompt] {
			t.Errorf("archived remote menu offers contextual creation: %+v", selection)
		}
		model.menu = rowMenu{}
		for _, action := range []string{keybind.Terminal, keybind.Prompt} {
			if got := model.action(action, KeyContext{}); got.Intent.Kind != NoAction {
				t.Errorf("archived remote key %s admitted action: %+v", action, selection)
			}
		}
		if selection.Kind != SessionRow && !offered[NewSession] {
			t.Errorf("remote global new session omitted: %+v", selection)
		}
	}
}

// → focuses a session by default and attaches when the arrow step says so.
// The legend names the one it does.
func TestStepInFollowsTheArrowStepValue(t *testing.T) {
	model := New(nil)
	model.Reconcile(Snapshot{Sessions: []Session{{ID: "agent", Name: "agent"}}})
	selection := Selection{Kind: SessionRow, SessionID: "agent"}
	model.Focus(selection)
	for _, tc := range []struct {
		attaches bool
		kind     ActionKind
		label    string
	}{{false, Focus, "focus"}, {true, Attach, "attach"}} {
		keys := KeyContext{ListKeys: keybind.DefaultList(), ArrowStep: true, StepInAttaches: tc.attaches}
		if decision := model.Key(tea.KeyMsg{Type: tea.KeyRight}, Frame{}, keys); decision.Intent.Kind != tc.kind || decision.Intent.Target != selection {
			t.Errorf("attaches = %v: → gave %+v, want %v on the session", tc.attaches, decision.Intent, tc.kind)
		}
		legend := model.RowLegend(LegendContext{
			ListKeys: keybind.DefaultList(), EnterFocuses: true, ArrowStep: true, StepInAttaches: tc.attaches,
			RowOK: true, Row: selection, Session: Session{ID: "agent"},
		})
		if !slices.Contains(legend.Pairs, [2]string{"→", tc.label}) {
			t.Errorf("attaches = %v: the legend lacks → %s: %v", tc.attaches, tc.label, legend.Pairs)
		}
	}
}

func TestRunMenuOffersRerunOnlyWhenPossible(t *testing.T) {
	model := New(nil)
	model.Reconcile(Snapshot{Runs: []Run{
		{ID: "f", Ticket: "jira:F-1", Status: "failed", CanRerun: true},
		{ID: "d", Ticket: "jira:D-1", Status: "done"},
	}})
	items := func(id string) []menuItem {
		return model.rowMenuItems(model.rows[model.rowIndex(Selection{Kind: RunRow, Run: id}.key())])
	}
	if got := items("f"); len(got) != 3 || got[1].label != "Re-run" || got[1].action != Restart {
		t.Fatalf("failed menu = %+v", got)
	}
	if got := items("d"); len(got) != 2 || got[0].label != "Open log" {
		t.Fatalf("done menu = %+v", got)
	}
}

func TestRunMenuMarksAndUnmarksTheTicketSolved(t *testing.T) {
	model := New(nil)
	model.Reconcile(Snapshot{Runs: []Run{
		{ID: "f", Ticket: "jira:F-1", Status: "failed", CanRerun: true},
		{ID: "s", Ticket: "jira:S-1", Status: "failed", CanRerun: true, Solved: true},
	}})
	last := func(id string) menuItem {
		items := model.rowMenuItems(model.rows[model.rowIndex(Selection{Kind: RunRow, Run: id}.key())])
		return items[len(items)-1]
	}
	if got := last("f"); got.label != "Mark solved" || got.action != MarkIdle {
		t.Fatalf("open Ticket's menu ends with %+v", got)
	}
	if got := last("s"); got.label != "Unmark solved" || got.action != MarkIdle {
		t.Fatalf("solved Ticket's menu ends with %+v", got)
	}
	if actionKind(keybind.MarkIdle) != MarkIdle {
		t.Fatal("the mark-handled key does not pick the menu's solved entry")
	}
}

func TestRunsMenusOfferTheFinishedToggleWhenThereAreFinished(t *testing.T) {
	model := New(nil)
	snapshot := Snapshot{Runs: []Run{{ID: "d", Ticket: "jira:D-1", Status: "done"}}}
	model.Reconcile(snapshot)
	header := func() []menuItem {
		return model.rowMenuItems(model.rows[model.rowIndex(Selection{Kind: RunsRow}.key())])
	}
	if got := header(); len(got) != 1 || got[0].label != "Fold" {
		t.Fatalf("header menu with nothing finished = %+v", got)
	}
	snapshot.RunsFinished = 4
	model.Reconcile(snapshot)
	if got := header(); len(got) != 2 || got[1].label != "Show finished" || got[1].action != ToggleFinishedRuns {
		t.Fatalf("header menu = %+v", got)
	}
	run := model.rowMenuItems(model.rows[model.rowIndex(Selection{Kind: RunRow, Run: "d"}.key())])
	if last := run[len(run)-1]; last.label != "Show finished" {
		t.Fatalf("run menu = %+v", run)
	}
	snapshot.RunsShowFinished = true
	model.Reconcile(snapshot)
	if got := header(); len(got) != 2 || got[1].label != "Hide finished" {
		t.Fatalf("header menu while shown = %+v", got)
	}
}

func TestRunsHeaderMenuFoldsAndUnfolds(t *testing.T) {
	model := New(nil)
	model.Reconcile(Snapshot{Runs: []Run{{ID: "d", Ticket: "jira:D-1", Status: "done"}}})
	for _, want := range []struct {
		label  string
		folded bool
	}{{"Fold", true}, {"Unfold", false}} {
		decision := model.openMenu(Selection{Kind: RunsRow}, 0, 0, false)
		if !model.menu.active || model.menu.items[0].label != want.label || model.menu.items[0].action != ToggleFold {
			t.Fatalf("menu = %+v, decision %+v; want %s first", model.menu.items, decision, want.label)
		}
		decision = model.runMenuItem(0)
		if model.hostFolds[RunsFold] != want.folded || decision.Intent.Kind != NoAction || len(decision.Mutations) != 1 {
			t.Fatalf("%s: folded %v, decision %+v", want.label, model.hostFolds[RunsFold], decision)
		}
	}
}

func TestANeedsHumanRunMenuOffersTheAnswer(t *testing.T) {
	model := New(nil)
	model.Reconcile(Snapshot{Runs: []Run{
		{ID: "asks", Ticket: "jira:A-1", Status: "needs-human", CanAnswer: true, CanRerun: true},
		{ID: "answered", Ticket: "jira:A-2", Status: "needs-human", CanAnswer: true, Answered: true},
		{ID: "failed", Ticket: "jira:A-3", Status: "failed", CanRerun: true},
	}})
	for _, tc := range []struct {
		id, want string
	}{{"asks", "Answer"}, {"answered", "Edit answer"}, {"failed", ""}} {
		model.Focus(Selection{Kind: RunRow, Run: tc.id})
		model.openMenu(Selection{Kind: RunRow, Run: tc.id}, 4, 5, false)
		found := -1
		for index, item := range model.menu.items {
			if item.action == Prompt {
				found = index
				if item.label != tc.want {
					t.Fatalf("%s menu offers %q, want %q", tc.id, item.label, tc.want)
				}
			}
		}
		if (found >= 0) != (tc.want != "") {
			t.Fatalf("%s menu answer item at %d, want %q", tc.id, found, tc.want)
		}
		if found >= 0 {
			if decision := model.runMenuItem(found); decision.Intent.Kind != Prompt || decision.Intent.Target.Run != tc.id {
				t.Fatalf("%s answer decision = %+v", tc.id, decision)
			}
		}
	}
}
