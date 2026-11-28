// Package rail is the session list: its rows, cursor, folds, search and
// filters, the drag reorder, the row menu, the legend and how each row is
// painted. It works on a copied snapshot of the workspace and returns typed
// intents and mutations; the root runs the store and tmux work they ask for.
package rail

import (
	"maps"
	"sort"
	"strings"
	"time"

	remote "github.com/YoanWai/agent-manager/internal/remote/value"
)

// Session is the copied subset of a workspace session that Rail uses for
// tree policy and presentation. Root keeps the concrete store record.
type Session struct {
	ID             string
	Name           string
	DisplayName    string
	Tool           string
	Cwd            string
	Group          string
	Status         string
	Archived       bool
	Acked          bool
	CreatedAt      time.Time
	LastStatusAt   time.Time
	LaunchAt       time.Time
	AgentSessionID string
	ParentID       string
	Prompt         string
	PaneLine       string
	PanePrompt     string
	Queued         int
	IsShell        bool
	Elsewhere      bool
	// AfterTurn is the archive or kill the agent asked for once its turn
	// ends; AfterTurnGlyph is root's mark for it.
	AfterTurn      string
	AfterTurnGlyph string
	// BotProfile is the Profile a Bot was started from, empty for any
	// other session.
	BotProfile string
	// Asking is whether the session published an Agent Surface the owner
	// has not answered; AskingGlyph is root's mark for it.
	Asking      bool
	AskingGlyph string
	// Pending is the action this manager has queued on the row, worded as
	// it is happening ("archiving"), until the action completes.
	Pending string
	// Babysitting is whether the session runs `fractory babysit` under its
	// babysit Session Mark.
	Babysitting bool
}

// Snapshot is an immutable-by-convention inventory copied at the root
// observation boundary. Reconcile copies every slice and map it retains.
type Snapshot struct {
	Sessions       []Session
	Groups         []string
	ArchivedGroups map[string]bool
	// Hosts are the SSH connections, listed after the local tree in this
	// order.
	Hosts []Host
	// Runs are Fractory's Runs, listed read-only after the local tree.
	Runs []Run
	// RunsFinished counts the finished Tickets; RunsShowFinished is true when
	// Runs lists them too.
	RunsFinished     int
	RunsShowFinished bool
	// Automations are the stored Automations, listed after the Runs.
	Automations []Automation
	// Bots are the stored Bot Profiles, listed after the Automations with
	// the live Bots each started.
	Bots []BotProfile
}

type RowKind uint8

const (
	SessionRow RowKind = iota
	GroupRow
	ConnectionRow
	// RunsRow is the Runs section's header; RunRow is one Run under it.
	RunsRow
	RunRow
	// AutomationsRow is the Automations section's header, AutomationRow one
	// Automation under it and FiringRow one of its firings.
	AutomationsRow
	AutomationRow
	FiringRow
	// BotsRow is the Bots section's header and BotProfileRow one Profile
	// under it; a live Bot under a Profile is a SessionRow.
	BotsRow
	BotProfileRow
)

// Selection names a row without exposing Rail's cursor or row storage.
// Host names the connection a remote row belongs to and is empty on every
// local row.
type Selection struct {
	Kind      RowKind
	SessionID string
	Group     string
	Host      string
	// Run is the Run's id on a RunRow.
	Run string
	// Automation is the Automation's id on an AutomationRow and a FiringRow;
	// Firing is the firing's record id on a FiringRow.
	Automation string
	Firing     string
	// Profile is the Bot Profile's id on a BotProfileRow.
	Profile string
}

func (s Selection) Remote() bool { return s.Host != "" }

func (s Selection) key() string {
	switch s.Kind {
	case RunsRow:
		return "runs"
	case RunRow:
		return "r:" + s.Run
	case AutomationsRow:
		return "automations"
	case AutomationRow:
		return "a:" + s.Automation
	case FiringRow:
		return "f:" + s.Automation + "/" + s.Firing
	case BotsRow:
		return "bots"
	case BotProfileRow:
		return "p:" + s.Profile
	case ConnectionRow:
		return "c:" + s.Host
	case GroupRow:
		if s.Host != "" {
			return "rg:" + remote.Ref{Host: s.Host, ID: s.Group}.String()
		}
		return "g:" + s.Group
	}
	if s.Host != "" {
		return "rs:" + remote.Ref{Host: s.Host, ID: s.SessionID}.String()
	}
	return "s:" + s.SessionID
}

// Row is a copied diagnostic and presentation value. Mutating a returned
// Row never changes the model.
type Row struct {
	Selection
	Depth  int
	Name   string
	Tool   string
	Status string
}

type SelectionChange struct {
	Changed  bool
	Previous Selection
	Current  Selection
}

// SelectionEffect tells root which concrete selection side effects the Rail
// policy requires. Search editing deliberately reports identity changes
// without requesting either effect; the old list rebuilt silently while the
// field was active.
type SelectionEffect uint8

const (
	SelectionEffectNone SelectionEffect = iota
	SelectionEffectPreview
	SelectionEffectFilter
)

type Decision struct {
	Consumed         bool
	SelectionChanged bool
	SelectionEffect  SelectionEffect
	Selection        Selection
	Intent           Intent
	Mutations        []Mutation
	Refresh          bool
	Error            string
	AutoScroll       *AutoScrollRequest
}

type treeRow struct {
	kind  RowKind
	group string
	depth int
	sess  Session
	host  string
	run   Run
	auto  Automation
	fire  Firing
	// bot is the Profile on its row and on each live Bot under it.
	bot BotProfile
}

func (r treeRow) selection() Selection {
	switch r.kind {
	case RunsRow:
		return Selection{Kind: RunsRow}
	case RunRow:
		return Selection{Kind: RunRow, Run: r.run.ID}
	case AutomationsRow:
		return Selection{Kind: AutomationsRow}
	case AutomationRow:
		return Selection{Kind: AutomationRow, Automation: r.auto.ID}
	case FiringRow:
		return Selection{Kind: FiringRow, Automation: r.auto.ID, Firing: r.fire.ID}
	case BotsRow:
		return Selection{Kind: BotsRow}
	case BotProfileRow:
		return Selection{Kind: BotProfileRow, Profile: r.bot.ID}
	case ConnectionRow:
		return Selection{Kind: ConnectionRow, Host: r.host}
	case GroupRow:
		return Selection{Kind: GroupRow, Group: r.group, Host: r.host}
	}
	return Selection{Kind: SessionRow, SessionID: r.sess.ID, Group: r.sess.Group, Host: r.host}
}

func (r treeRow) key() string { return r.selection().key() }

func (r treeRow) isRoot() bool { return r.kind == GroupRow && r.group == "" && r.host == "" }

// folds reports whether the row opens and closes: a group, a connection,
// the Runs, Automations or Bots header, an Automation over its firings or
// a Bot Profile over its live Bots.
func (r treeRow) folds() bool {
	switch r.kind {
	case GroupRow, ConnectionRow, RunsRow, AutomationsRow, AutomationRow, BotsRow, BotProfileRow:
		return true
	}
	return false
}

type statusFilter uint8

const (
	statusFilterAll statusFilter = iota
	statusFilterAttention
)

type Model struct {
	snapshot        Snapshot
	rows            []treeRow
	cursor          int
	reorder         reorderState
	menu            rowMenu
	lifts           int
	listClickAt     time.Time
	listClickKey    string
	clickFocusKey   string
	showArchived    bool
	hideEmptyGroups bool
	filter          statusFilter
	collapsed       map[string]bool
	hostFolds       map[HostFold]bool
	// openAutomations holds the Automations whose firings are listed; an
	// Automation starts folded, so the section stays one row each.
	openAutomations map[string]bool
	// openProfiles holds the Bot Profiles whose live Bots are listed; a
	// Profile starts folded, like an Automation.
	openProfiles map[string]bool
	search       string
	searching    bool
}

func New(collapsed []string) Model {
	m := Model{collapsed: make(map[string]bool, len(collapsed)), hostFolds: map[HostFold]bool{}, openAutomations: map[string]bool{}, openProfiles: map[string]bool{}}
	for _, path := range collapsed {
		m.collapsed[path] = true
	}
	return m
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	copyOf := Snapshot{
		Sessions:       append([]Session(nil), snapshot.Sessions...),
		Groups:         append([]string(nil), snapshot.Groups...),
		ArchivedGroups: make(map[string]bool, len(snapshot.ArchivedGroups)),
		Hosts:          make([]Host, len(snapshot.Hosts)),
		Runs:           append([]Run(nil), snapshot.Runs...),
		Automations:    make([]Automation, len(snapshot.Automations)),
		Bots:           append([]BotProfile(nil), snapshot.Bots...),

		RunsFinished:     snapshot.RunsFinished,
		RunsShowFinished: snapshot.RunsShowFinished,
	}
	for path, archived := range snapshot.ArchivedGroups {
		copyOf.ArchivedGroups[path] = archived
	}
	for i, automation := range snapshot.Automations {
		automation.Firings = append([]Firing(nil), automation.Firings...)
		copyOf.Automations[i] = automation
	}
	for i, host := range snapshot.Hosts {
		host.Sessions = append([]Session(nil), host.Sessions...)
		host.Groups = append([]string(nil), host.Groups...)
		host.ArchivedGroups = maps.Clone(host.ArchivedGroups)
		copyOf.Hosts[i] = host
	}
	return copyOf
}

func (m *Model) Reconcile(snapshot Snapshot) SelectionChange {
	previous, previousOK := m.Selected()
	m.snapshot = cloneSnapshot(snapshot)
	m.rebuildRows()
	current, currentOK := m.Selected()
	return SelectionChange{
		Changed:  previousOK != currentOK || previous != current,
		Previous: previous,
		Current:  current,
	}
}

func (m Model) Selected() (Selection, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return Selection{}, false
	}
	return m.rows[m.cursor].selection(), true
}

// SelectedSession is the selected local session; a remote row is none.
func (m Model) SelectedSession() (string, bool) {
	selected, ok := m.Selected()
	return selected.SessionID, ok && selected.Kind == SessionRow && !selected.Remote()
}

// SelectedGroup is the selected local group; a remote row is none.
func (m Model) SelectedGroup() (string, bool) {
	selected, ok := m.Selected()
	return selected.Group, ok && selected.Kind == GroupRow && !selected.Remote()
}

func (m Model) Rows() []Row {
	rows := make([]Row, len(m.rows))
	for i, row := range m.rows {
		rows[i] = Row{
			Selection: row.selection(),
			Depth:     row.depth,
			Name:      row.sess.Name,
			Tool:      row.sess.Tool,
			Status:    row.sess.Status,
		}
		switch {
		case row.kind == RunsRow:
			rows[i].Name = "runs"
		case row.kind == RunRow:
			rows[i].Name = row.run.Ticket
		case row.kind == AutomationsRow:
			rows[i].Name = "automations"
		case row.kind == AutomationRow:
			rows[i].Name = row.auto.Name
		case row.kind == FiringRow:
			rows[i].Name = row.fire.When
		case row.kind == BotsRow:
			rows[i].Name = "bots"
		case row.kind == BotProfileRow:
			rows[i].Name = row.bot.Name
			rows[i].Tool = row.bot.Tool
		case row.kind == ConnectionRow:
			rows[i].Name = row.host
		case row.isRoot():
			rows[i].Name = "root"
		case row.kind == GroupRow:
			rows[i].Name = BaseName(row.group)
		}
	}
	return rows
}

func (m *Model) Move(delta int, wrap bool) Decision {
	if len(m.rows) == 0 || delta == 0 {
		return Decision{Consumed: true}
	}
	previous := m.cursor
	next := m.cursor + delta
	if wrap {
		if next < 0 {
			next = len(m.rows) - 1
		}
		if next >= len(m.rows) {
			next = 0
		}
	} else {
		next = min(max(next, 0), len(m.rows)-1)
	}
	m.cursor = next
	decision := Decision{Consumed: true, SelectionChanged: previous != next}
	if decision.SelectionChanged {
		decision.SelectionEffect = SelectionEffectPreview
	}
	return decision
}

func (m *Model) FocusSession(id string) bool {
	for i, row := range m.rows {
		if row.kind == SessionRow && row.host == "" && row.sess.ID == id {
			m.cursor = i
			return true
		}
	}
	return false
}

func (m *Model) Focus(selection Selection) bool {
	for i, row := range m.rows {
		if row.selection() == selection {
			m.cursor = i
			return true
		}
	}
	return false
}

func (m Model) row() (treeRow, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return treeRow{}, false
	}
	return m.rows[m.cursor], true
}

func (m Model) listedSessions() []Session {
	// Folding hides rows without narrowing a group's logical action scope.
	m.collapsed = nil
	var sessions []Session
	for _, row := range m.treeRows("", m.snapshot.Sessions, m.snapshot.Groups, m.snapshot.ArchivedGroups, 0) {
		if row.kind == SessionRow {
			sessions = append(sessions, row.sess)
		}
	}
	return sessions
}

func (m Model) listed(host string, sessions []Session) []Session {
	archivedGroups := m.snapshot.ArchivedGroups
	if host != "" {
		for _, remote := range m.snapshot.Hosts {
			if remote.Name == host {
				archivedGroups = remote.ArchivedGroups
				break
			}
		}
	}
	return m.eligibleSessions(host, sessions, archivedGroups)
}

// eligibleSessions is what the view and the status filter keep of one host's
// sessions; the selected session stays while the filter is on.
func (m Model) eligibleSessions(host string, sessions []Session, archivedGroups map[string]bool) []Session {
	visible := make([]Session, 0, len(sessions))
	for _, session := range sessions {
		if session.Archived == m.showArchived && (m.showArchived || !effectivelyArchived(archivedGroups, session.Group)) {
			visible = append(visible, session)
		}
	}
	if m.filter == statusFilterAll {
		return visible
	}
	held, _ := m.Selected()
	listed := make([]Session, 0, len(visible))
	for _, session := range visible {
		if attentionStatus(session.Status) || session.Asking || held.Kind == SessionRow && held.Host == host && held.SessionID == session.ID {
			listed = append(listed, session)
		}
	}
	return listed
}

func attentionStatus(state string) bool {
	return state == "waiting" || state == "finished" || state == "errored"
}

func (m *Model) rebuildRows() {
	previous := ""
	if row, ok := m.row(); ok {
		previous = row.key()
	}
	rows := []treeRow{{kind: GroupRow}}
	rows = append(rows, m.treeRows("", m.treeSessions(), m.snapshot.Groups, m.snapshot.ArchivedGroups, 0)...)
	rows = append(rows, m.runRows()...)
	rows = append(rows, m.automationRows()...)
	rows = append(rows, m.botRows()...)
	for _, host := range m.snapshot.Hosts {
		rows = append(rows, treeRow{kind: ConnectionRow, host: host.Name})
		if m.honorFolds() && m.folded(host.Name, "") {
			continue
		}
		rows = append(rows, m.treeRows(host.Name, host.Sessions, host.Groups, host.ArchivedGroups, 1)...)
	}

	m.rows = rows
	if previous != "" {
		found := false
		for i, row := range rows {
			if row.key() == previous {
				m.cursor, found = i, true
				break
			}
		}
		// A rail emptied under the cursor rests on the root, not on the
		// empty Bots header that happens to sit where the row was.
		if !found && m.nothingListed() {
			m.cursor = 0
		}
	} else if m.cursor == 0 && len(rows) > 1 && rows[0].isRoot() && rows[1].kind != BotsRow {
		m.cursor = 1
	}
	if m.cursor >= len(rows) {
		m.cursor = len(rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m Model) pruned() bool {
	return strings.TrimSpace(m.search) != "" || m.filter != statusFilterAll
}

func (m Model) honorFolds() bool { return !m.pruned() && !m.showArchived }

// treeRows lays out one host's groups and sessions from depth on: the
// local host's under the root row, a connection's under its own row.
func (m Model) treeRows(host string, all []Session, groups []string, archivedGroups map[string]bool, depth int) []treeRow {
	query := strings.ToLower(strings.TrimSpace(m.search))
	listed := m.eligibleSessions(host, all, archivedGroups)
	listedIDs := make(map[string]bool, len(listed))
	for _, session := range listed {
		listedIDs[session.ID] = true
	}
	byID := make(map[string]Session, len(all))
	for _, session := range all {
		byID[session.ID] = session
	}
	matched := make(map[string]bool, len(listed))
	for _, session := range listed {
		if query == "" || matchesSearch(session, query) {
			matched[session.ID] = true
		}
	}
	carried := map[string]bool{}
	for _, session := range listed {
		if matched[session.ID] && session.ParentID != "" && listedIDs[session.ParentID] {
			carried[session.ParentID] = true
		}
	}
	sessionsByGroup := map[string][]Session{}
	childrenByParent := map[string][]Session{}
	for _, session := range listed {
		if session.ParentID != "" {
			if _, ok := byID[session.ParentID]; ok {
				if matched[session.ID] {
					childrenByParent[session.ParentID] = append(childrenByParent[session.ParentID], session)
				}
				continue
			}
		}
		if matched[session.ID] || carried[session.ID] {
			sessionsByGroup[session.Group] = append(sessionsByGroup[session.Group], session)
		}
	}
	walked := map[string]bool{}
	for _, sessions := range sessionsByGroup {
		for _, session := range sessions {
			walked[session.ID] = true
		}
	}
	orphaned := map[string]bool{}
	for _, session := range listed {
		if _, nested := childrenByParent[session.ParentID]; !nested || walked[session.ParentID] || !matched[session.ID] {
			continue
		}
		sessionsByGroup[session.Group] = append(sessionsByGroup[session.Group], session)
		orphaned[session.ParentID] = true
	}
	for parentID := range orphaned {
		delete(childrenByParent, parentID)
	}

	paths := groupClosure(groups, all)
	if m.showArchived {
		kept := pathsWithSessions(paths, sessionsByGroup)
		for path := range paths {
			if effectivelyArchived(archivedGroups, path) {
				addWithAncestors(kept, path)
			}
		}
		paths = kept
	} else {
		for path := range paths {
			if effectivelyArchived(archivedGroups, path) {
				delete(paths, path)
			}
		}
		if m.pruned() {
			paths = pathsWithSessions(paths, sessionsByGroup)
		}
	}
	if m.hideEmptyGroups && !m.showArchived {
		paths = pathsWithSessions(paths, sessionsByGroup)
	}
	children := childIndex(paths, groups)
	honorFolds := m.honorFolds()

	rows := make([]treeRow, 0, len(all)+len(paths))
	appendSession := func(session Session, depth int) {
		rows = append(rows, treeRow{kind: SessionRow, sess: session, depth: depth, host: host})
		for _, child := range childrenByParent[session.ID] {
			rows = append(rows, treeRow{kind: SessionRow, sess: child, depth: depth + 1, host: host})
		}
	}
	for _, session := range sessionsByGroup[""] {
		appendSession(session, depth)
	}
	var walk func(string, int)
	walk = func(path string, depth int) {
		rows = append(rows, treeRow{kind: GroupRow, group: path, depth: depth, host: host})
		if honorFolds && m.folded(host, path) {
			return
		}
		for _, session := range sessionsByGroup[path] {
			appendSession(session, depth+1)
		}
		for _, child := range children[path] {
			walk(child, depth+1)
		}
	}
	for _, root := range children[""] {
		walk(root, depth)
	}
	return rows
}

func groupClosure(groups []string, sessions []Session) map[string]bool {
	paths := map[string]bool{}
	add := func(path string) {
		for path != "" {
			paths[path] = true
			path = ParentGroup(path)
		}
	}
	for _, group := range groups {
		add(group)
	}
	for _, session := range sessions {
		add(session.Group)
	}
	return paths
}

func effectivelyArchived(archived map[string]bool, path string) bool {
	for path != "" {
		if archived[path] {
			return true
		}
		path = ParentGroup(path)
	}
	return false
}

func addWithAncestors(set map[string]bool, path string) {
	for path != "" {
		set[path] = true
		path = ParentGroup(path)
	}
}

func pathsWithSessions(paths map[string]bool, sessions map[string][]Session) map[string]bool {
	kept := map[string]bool{}
	for group := range sessions {
		// A walk stops at a kept path: the walk that kept it already went
		// through every ancestor above it.
		for path := group; path != "" && !kept[path]; path = ParentGroup(path) {
			if paths[path] {
				kept[path] = true
			}
		}
	}
	return kept
}

func childIndex(paths map[string]bool, ordered []string) map[string][]string {
	rank := make(map[string]int, len(ordered))
	for i, path := range ordered {
		rank[path] = i
	}
	children := map[string][]string{}
	for path := range paths {
		parent := ParentGroup(path)
		children[parent] = append(children[parent], path)
	}
	for _, siblings := range children {
		sort.SliceStable(siblings, func(i, j int) bool {
			ri, iKnown := rank[siblings[i]]
			rj, jKnown := rank[siblings[j]]
			if iKnown && jKnown {
				return ri < rj
			}
			if iKnown != jKnown {
				return iKnown
			}
			return siblings[i] < siblings[j]
		})
	}
	return children
}

// ParentGroup, BaseName and InGroupSubtree read slash-separated group
// paths, where "" is the root group.
func ParentGroup(group string) string {
	if index := strings.LastIndex(group, "/"); index >= 0 {
		return group[:index]
	}
	return ""
}

func BaseName(path string) string {
	if index := strings.LastIndex(path, "/"); index >= 0 {
		return path[index+1:]
	}
	return path
}

func InGroupSubtree(group, parent string) bool {
	return group == parent || strings.HasPrefix(group, parent+"/")
}

func matchesSearch(session Session, query string) bool {
	return strings.Contains(strings.ToLower(session.Name), query) ||
		strings.Contains(strings.ToLower(session.Tool), query) ||
		strings.Contains(strings.ToLower(session.Group), query) ||
		strings.Contains(strings.ToLower(session.Status), query)
}
