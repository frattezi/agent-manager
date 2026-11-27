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

**Agent Surface**:
A screen an agent asked for rather than one the manager composed: the agent describes what the
owner should see or decide, and the manager draws it. It is never painted inside a Session's
Pane, which belongs to that Session's CLI.
_Avoid_: Surface, widget, card, A2UI surface, agent UI

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

## Agents reaching each other

Sessions in one workspace coordinate by messaging each other. The words below separate
a message that expects nothing from one that expects an answer, and a Session that was
merely started from one that owes a result.

**Message**:
One item in a recipient Session's inbox, typed into that Session once its Pane is at
rest. The unit of everything in this section: a Tell, an Ask and an Answer are all
Messages differing in what they carry.
_Avoid_: Notification, event, ping

**Tell**:
A Message that expects nothing back.
_Avoid_: Handoff, fire-and-forget

**Ask**:
A Message that expects an Answer, and so opens a Thread. Asking does not oblige the
sender to wait; an Ask nobody waits on is delivered like a Tell.
_Avoid_: Question, request, query

**Answer**:
A Message replying to an Ask on its Thread. Alone among Messages, an Answer can be
taken by the asker's waiting call and returned as that call's result, in which case it
never reaches a Pane.
_Avoid_: Reply, response, result

**Thread**:
The identity shared by an Ask and the Answers to it. What makes "answered" a fact
rather than an inference, and what a waiting Session waits on.
_Avoid_: Conversation, channel, topic

**Coordinator**:
A Session that dispatched another. Not a role a Session holds but a relation between
two: the same Session is a Coordinator to those it dispatched and a Worker to whoever
dispatched it.
_Avoid_: Parent, supervisor, orchestrator, manager

**Worker**:
A Session that was dispatched and owes its Coordinator a Result. A Session started
without that obligation is just a Session.
_Avoid_: Child, subagent

**Dispatch**:
Creating a Worker with the obligation to report a Result, on a Thread minted for it.
Distinct from starting a Session, which carries no obligation.
_Avoid_: Spawn, fork, delegate

**Result**:
What a Worker owes its Coordinator: one summary the Worker writes, and the facts the
manager assembles itself. It travels as the Answer on the Dispatch's Thread, and a
Worker that dies has one written for it.
_Avoid_: Report, output, return value

**Task**:
One item on the shared list, which any Session may create and any may claim, and which
may depend on others. A Task is the work; a Thread is a conversation about work.
_Avoid_: Job, issue

**Reservation**:
An advisory, expiring lease a Session holds on a path, so Sessions sharing a checkout
can see who is editing what. Never enforced against a writer.
_Avoid_: Lock, mutex

## Fractory Runs

**Ticket**:
One unit of work recorded in a ticket source, which the factory raises Runs for. A Ticket is
work the factory owns; a Task is work the Sessions share.
_Avoid_: Issue, card, story

**Run**:
One attempt by the factory at a Ticket: it is raised, it works on a branch of its own, and it
ends done, failed, or waiting on a human.
_Avoid_: Build, job, execution, attempt

**Project**:
The repository a Run's work belongs to. One owner has many Projects; neither the owner nor the
forge is part of a Project's name.
_Avoid_: Repo, repository, workspace, codebase

**Run Title**:
The one-line, human-readable description of what a Run is doing, written for someone scanning a
list of them. It belongs to the Ticket the Run was raised for.
_Avoid_: Task description, summary, slug, branch name

**Ticket Snapshot**:
What the ticket source said about a Project's Tickets at one moment, recorded by the factory so
the manager can show Tickets without asking the tracker itself. It is a reading, never the truth:
it carries the instant it was taken, and it is always behind the tracker.
_Avoid_: Ticket cache, index, mirror, sync

**Ticket Ref**:
The identifier a ticket source gives a Ticket, carrying the forge, the owner, the Project and
the number. It identifies a Ticket; it is not what a reader is shown.
_Avoid_: Ticket id, ticket key, ticket name

## Bots

**Bot Profile**:
A reusable configuration the owner can start an agent from: who it is, what it specializes in,
which CLI and model it runs on, how it is shown, and where it remembers. It is configuration the
manager stores, not a process; starting one costs nothing until a Bot is spawned from it.
_Avoid_: Bot config, template, persona, role, preset

**Bot**:
One live Session spawned from a Bot Profile, which the owner talks to. Several Bots may run from
one Profile at once, each attending to something different, and they share the Profile's memory.
_Avoid_: Agent, instance, assistant, worker

**Specialization**:
What a Bot Profile knows that a plain Session does not, held as a committed skill rather than as
text in a configuration field, so it is reviewed and versioned like any other code.
_Avoid_: Personality, prompt, instructions, character
