# Agent Manager

The workspace a person runs their AI coding CLIs in: it owns the sessions, shows
what each agent is doing, and hands the terminal over when they want to drive one
directly. This glossary fixes the vocabulary for those concepts; it holds no
implementation detail.

## Language

**Session**:
One agent process the manager owns, running a single CLI in a single place in the
workspace.
_Avoid_: Agent, window, instance, tab

**Pane**:
The region a Session's CLI draws into, and the only surface its own output appears on.
_Avoid_: Terminal, screen, buffer

**Frame**:
The complete picture the manager presents at one moment, composed as a whole rather
than patched in parts.
_Avoid_: View, screen, render, draw

**Preview**:
A read-only look at a Session's Pane shown beside the workspace, so a person can see
an agent's state without leaving the list.
_Avoid_: Thumbnail, snapshot, peek

## Driving a session

The two ways a person reaches a Session's CLI. They are different concepts, not two
settings of one concept, and they behave differently under load; naming them
precisely is the point of this section.

**Attach**:
Handing the whole terminal to a Session, so the person is talking to the CLI itself
and the manager is not between them.
_Avoid_: Full screen, open, enter, attached mode

**Focus mode**:
Typing into a Session from inside the workspace, with the manager still on screen and
relaying what the person types.
_Avoid_: Inline mode, interactive preview, focused preview, editing the preview

## Automations

**Automation**:
A stored configuration that starts work on a schedule, without anyone asking. It names
where the work runs, which CLI runs it, and under which environment, and it is owned and
remembered by the manager rather than by the tool it starts.
_Avoid_: Job, cronjob, task, routine, hook, trigger

**Agent Automation**:
An Automation whose work is given to an agent as a prompt. The agent works alone and nobody
talks to it while it runs.
_Avoid_: Chat automation, prompt job, conversation

**Script Automation**:
An Automation whose work is a command: it runs, it ends, and what it printed stays readable
afterwards.
_Avoid_: Shell job, command task, bash hook

**Debug Session**:
A session the owner starts from a finished Automation Run to look into what it did. It runs
where they are, not where the Automation ran, and it begins knowing which Automation produced
the run it is asked about.
_Avoid_: Handoff, retry, attach

**Mode**:
The level of human oversight the work requires, either Work or Personal. Fractory fixes a
repository's Mode; an Automation carries its own, because one repository can hold both Work
and Personal Automations.
_Avoid_: Profile, environment, account
