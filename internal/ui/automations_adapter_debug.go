package ui

import (
	"github.com/YoanWai/agent-manager/internal/ui/automations"
	uilaunch "github.com/YoanWai/agent-manager/internal/ui/spawn/launch"
	tea "github.com/charmbracelet/bubbletea"
)

// openDebugSession spawns a Debug Session on a finished firing: an ordinary
// chat in the Automation's repository, under that project's group, which is
// created first when the repository has none. A Script Automation records no
// CLI, so its Debug Session opens on the picker's default.
func (m *Model) openDebugSession(entry automations.Entry, record automations.Record) tea.Cmd {
	if entry.Err != nil {
		m.reportErr("this Automation cannot be read, so it has no Debug Session: " + entry.Err.Error())
		return nil
	}
	debug, err := automations.DebugSession(entry.Config, record)
	if err != nil {
		m.reportErr(err.Error())
		return nil
	}
	tool := entry.Config.Tool
	if tool == "" {
		tools, index := m.cachedDefaultToolSelection()
		if len(tools) == 0 {
			m.reportErr("no CLI is enabled to open a Debug Session with")
			return nil
		}
		tool = tools[index]
	}
	if _, known := m.services.cfg.Tools[tool]; !known {
		m.reportErr("this Automation runs " + tool + ", which this manager no longer knows, so it has no Debug Session")
		return nil
	}
	group, create := automations.ProjectGroup(debug.Dir, m.debugGroups())
	m.clearErr()
	if create {
		m.enqueueEffect(uilaunch.GroupRequest{Path: group, RawDir: debug.Dir}, 0, false)
	}
	defaults := m.spawnDefaults(group)
	m.dispatchSpawn(uilaunch.Request{
		Kind:        uilaunch.Debug,
		ToolName:    tool,
		Name:        debug.Name,
		Group:       group,
		Prompt:      debug.Prompt,
		Pane:        defaults.Pane,
		Base:        defaults.Base,
		Environment: defaults.Env,
		RawDir:      debug.Dir,
	})
	return m.nextEffectCmd()
}

// debugGroups is every local group with its default directory, the ones a
// Debug Session may be placed under.
func (m *Model) debugGroups() []automations.Group {
	groups := make([]automations.Group, 0, len(m.workspace.groups))
	for _, path := range m.workspace.groups {
		groups = append(groups, automations.Group{Path: path, Dir: m.workspace.groupPaths[path], Archived: m.groupEffectivelyArchived(path)})
	}
	return groups
}
