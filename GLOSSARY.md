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
