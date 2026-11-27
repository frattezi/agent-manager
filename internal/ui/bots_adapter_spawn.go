package ui

import (
	"path/filepath"
	"strings"

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
// A Profile with no directory gets its own under the Bots root. The
// directory and its memory marker are readied first, with the skill file of
// a Specialization on a CLI that appends one to its system prompt; a
// Specialization with no skill file starts nothing.
func (m *Model) spawnBot(profile bots.Profile) tea.Cmd {
	tool, known := m.services.cfg.Tools[profile.Tool]
	if !known || tool.Shell {
		m.reportErr("the Bot Profile " + profile.Name + " runs " + profile.Tool + ", which is not an agent CLI this manager knows")
		return nil
	}
	m.clearErr()
	request := bots.PrepareRequest{Profile: profile, Owned: strings.TrimSpace(profile.Dir) == ""}
	if request.Owned {
		request.Dir = bots.OwnDir(m.env.botsDir, profile)
	} else {
		request.Dir = m.capturedAbsolutePath(profile.Dir, m.env.workDir)
	}
	if profile.Specialization != "" && tool.AppendSystemPromptFlag != "" {
		request.Specialize = true
		request.Roots = bots.SkillRoots(m.env.factoryDir, m.env.homeDir)
	}
	m.enqueueEffect(request, 0, false)
	return m.nextEffectCmd()
}

// botsRoot is the directory each Profile with no directory of its own gets
// one under.
func botsRoot(home string) string {
	return filepath.Join(home, "Work", "Bots")
}

// applyBotPrepare launches the Bot the preparation was for, with the skill
// file on its command line when it has one, or reports why it cannot start.
func (m *Model) applyBotPrepare(request bots.PrepareRequest, prepared bots.Prepared, err error) tea.Cmd {
	profile := request.Profile
	if err != nil {
		m.reportErr("the Bot Profile " + profile.Name + " cannot start: " + err.Error())
		return nil
	}
	flags := profile.Flags
	if prepared.Skill != "" {
		flag := m.services.cfg.Tools[profile.Tool].AppendSystemPromptFlag
		flags = bots.SpecializedFlags(profile.Flags, flag, tmux.ShellQuote(prepared.Skill))
	}
	m.launchBot(profile, prepared.Dir, flags)
	return m.nextEffectCmd()
}

// launchBot goes down the spawn path every session takes, on an empty
// composer; only the directory, the flags and the Profile id it records set
// it apart. The flags are stored as the session's launch flags, so a revive
// keeps them.
func (m *Model) launchBot(profile bots.Profile, dir, flags string) {
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
