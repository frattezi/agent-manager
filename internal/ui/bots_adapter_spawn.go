package ui

import (
	"github.com/YoanWai/agent-manager/internal/config"
	"github.com/YoanWai/agent-manager/internal/tmux"
	"github.com/YoanWai/agent-manager/internal/ui/automations"
	"github.com/YoanWai/agent-manager/internal/ui/bots"
	uieffects "github.com/YoanWai/agent-manager/internal/ui/effects"
	uilaunch "github.com/YoanWai/agent-manager/internal/ui/spawn/launch"
	tea "github.com/charmbracelet/bubbletea"
)

// spawnBot starts a Bot from profile: an ordinary session in the Profile's
// directory, under that project's group, which is created first when the
// directory has none, running the Profile's tool and model with its flags.
// A Specialization on a CLI that appends a file to its system prompt is
// looked up first, and a Specialization with no skill file starts nothing.
func (m *Model) spawnBot(profile bots.Profile) tea.Cmd {
	tool, known := m.services.cfg.Tools[profile.Tool]
	if !known || tool.Shell {
		m.reportErr("the Bot Profile " + profile.Name + " runs " + profile.Tool + ", which is not an agent CLI this manager knows")
		return nil
	}
	m.clearErr()
	if profile.Specialization == "" || tool.AppendSystemPromptFlag == "" {
		m.launchBot(profile, profile.Flags)
		return m.nextEffectCmd()
	}
	m.enqueueEffect(bots.SpecializeRequest{
		Profile: profile,
		Roots:   bots.SkillRoots(m.env.factoryDir, m.env.homeDir),
	}, 0, false)
	return m.nextEffectCmd()
}

// applyBotSpecialize launches the Bot the lookup was for with the skill file
// on its command line, or reports the Profile as misconfigured.
func (m *Model) applyBotSpecialize(request bots.SpecializeRequest, path string, err error) tea.Cmd {
	profile := request.Profile
	if err != nil {
		m.reportErr("the Bot Profile " + profile.Name + " cannot start: " + err.Error())
		return nil
	}
	flag := m.services.cfg.Tools[profile.Tool].AppendSystemPromptFlag
	m.launchBot(profile, bots.SpecializedFlags(profile.Flags, flag, tmux.ShellQuote(path)))
	return m.nextEffectCmd()
}

// launchBot goes down the spawn path every session takes; only the first
// prompt, the flags and the Profile id it records set it apart. The flags
// are stored as the session's launch flags, so a revive keeps them.
func (m *Model) launchBot(profile bots.Profile, flags string) {
	dir := m.capturedAbsolutePath(profile.Dir, m.env.workDir)
	group, create := automations.ProjectGroup(dir, m.debugGroups())
	bot := bots.Spawn(profile, m.botNamesTaken())
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
		Choice:      config.Choice{Model: profile.Model, Flags: flags},
		Pane:        defaults.Pane,
		Base:        defaults.Base,
		Environment: defaults.Env,
		RawDir:      dir,
		BotProfile:  profile.ID,
	})
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
