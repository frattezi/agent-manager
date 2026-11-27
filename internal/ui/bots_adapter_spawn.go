package ui

import (
	"github.com/YoanWai/agent-manager/internal/config"
	"github.com/YoanWai/agent-manager/internal/ui/automations"
	"github.com/YoanWai/agent-manager/internal/ui/bots"
	uieffects "github.com/YoanWai/agent-manager/internal/ui/effects"
	uilaunch "github.com/YoanWai/agent-manager/internal/ui/spawn/launch"
	tea "github.com/charmbracelet/bubbletea"
)

// spawnBot starts a Bot from profile: an ordinary session in the Profile's
// directory, under that project's group, which is created first when the
// directory has none, running the Profile's tool and model with its flags.
// It goes down the spawn path every session takes; only the first prompt and
// the Profile id it records set it apart.
func (m *Model) spawnBot(profile bots.Profile) tea.Cmd {
	tool, known := m.services.cfg.Tools[profile.Tool]
	if !known || tool.Shell {
		m.reportErr("the Bot Profile " + profile.Name + " runs " + profile.Tool + ", which is not an agent CLI this manager knows")
		return nil
	}
	dir := m.capturedAbsolutePath(profile.Dir, m.env.workDir)
	group, create := automations.ProjectGroup(dir, m.debugGroups())
	bot := bots.Spawn(profile, m.botNamesTaken())
	m.clearErr()
	if create {
		m.enqueueEffect(uilaunch.GroupRequest{Path: group, RawDir: dir}, 0, false)
	}
	defaults := m.spawnDefaults(group)
	m.dispatchSpawn(uilaunch.Request{
		Kind:        uilaunch.Bot,
		ToolName:    profile.Tool,
		Name:        bot.Name,
		Group:       group,
		Prompt:      bot.Prompt,
		Choice:      config.Choice{Model: profile.Model, Flags: profile.Flags},
		Pane:        defaults.Pane,
		Base:        defaults.Base,
		Environment: defaults.Env,
		RawDir:      dir,
		BotProfile:  profile.ID,
	})
	return m.nextEffectCmd()
}

// botNamesTaken is every live session's name and every Bot's still being
// spawned, so two quick spawns from one Profile are not given the same one.
func (m *Model) botNamesTaken() []string {
	names := make([]string, 0, len(m.workspace.sessions))
	for _, sess := range m.workspace.sessions {
		names = append(names, sess.Name)
	}
	for _, job := range append([]*uieffects.Job{m.effects.Main.Active}, m.effects.Main.Pending...) {
		if job == nil {
			continue
		}
		if pending, ok := job.Request.(uilaunch.Request); ok && pending.Kind == uilaunch.Bot {
			names = append(names, pending.Name)
		}
	}
	return names
}
