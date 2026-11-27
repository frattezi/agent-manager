# Spec 0007: A grilling round answered in a form

Status: accepted (2026-11-27). Vocabulary: [GLOSSARY.md](../../GLOSSARY.md).

## Goal

Grilling a Goal into a Spec is the owner's highest-value conversation and the manager shows
none of it. A round arrives as prose in a pane: numbered questions, each with a recommended
answer, which the owner answers by typing back the numbers they disagree with. The round
scrolls away as the agent works, the decisions it writes to the glossary and the ADRs are
invisible until the owner opens those files, and the progress of a Goal toward a Spec is
something only the agent knows.

The owner wants a round presented as what it is: a form. The questions, the recommended
answer already chosen for each, somewhere to say otherwise, and one key to send it. They
want the rounds kept, so a Goal's progress toward a Spec can be read back afterwards
instead of scrolled for. Nothing about how the agent interviews changes; only where the
owner reads the questions and how the answer gets back.

## Non-goals

- **Drawing inside a session's pane.** The pane belongs to that session's CLI, which
  repaints it. An Agent Surface is drawn in a surface the manager owns
  ([ADR 0007](../adr/0007-agent-authored-surfaces-are-a2ui.md)).
- **Adopting `a2tea`.** It renders A2UI already, and needs Bubble Tea v2, which this fork
  does not run. The trigger for adopting it is in ADR 0007; this Spec renders the subset it
  needs with the lipgloss and bubbles already in `go.mod`.
- **The whole A2UI catalog.** A subset covers a grilling round. A component outside it
  parses and is skipped with a reason, so a richer surface degrades rather than failing.
- **Changing how grilling works.** The skill decides what to ask, in what order, and what
  to recommend. This Spec moves the presentation, not the interview.
- **Deciding anything on the owner's behalf.** A surface never submits itself, never times
  out into a default, and an unanswered round stays unanswered.
- **The Tickets view.** [Spec 0006](0006-tickets-visible-before-their-first-run.md),
  accepted and unscheduled.
- **Anything in Fractory.** A Run that stops for a human keeps the path it has today. Moving
  `needs-human` onto Agent Surfaces is the obvious next use and is deliberately not bundled
  here.
- **A surface published by anything but a session.** Only a session the manager owns can
  publish, because the answer travels back into that session's conversation.

## Solution

**A session publishes a surface.** A session running the grilling skill describes its round
in A2UI and publishes it through the manager, as `agent-manager surface publish` or through
the matching MCP tool — the two fronts onto one command layer that `automations` and the
coordination commands already use. The manager validates the document and stores it against
the calling session. A refusal names what is wrong in terms the agent can act on, so a
malformed surface is something the agent fixes and republishes rather than a dead end.

**The row says someone is asking.** The session's row carries a mark while a surface is
unanswered, and the attention filter (`w`) keeps that session, so one key narrows the list
to everything waiting on the owner — a Run that stopped, a session that finished, and now a
round to answer. Nothing opens itself: a surface that seized the screen would land while
the owner is mid-sentence in another session, which is the same reason a message waits for
a pane to be at rest.

**The owner opens it with a key** and the manager draws the round over the list: each
question with its recommended answer already chosen, the choices steppable, a field for
anything the choices do not cover, and a send key. `esc` leaves it unanswered and the mark
stays. The surface is read from the top of the stack like every other card, so the keys
underneath it are not reachable by accident.

**The answer goes back as a typed turn.** Sending hands the answer to the delivery path
that already exists for messages between agents: queued, typed into the session once its
pane is at rest, never while it is working. The agent reads it as the owner's answer to
that round and carries on. The skill needs no polling and spends no tokens waiting.

**The rounds are kept.** An answered surface is retained with its answer against the
session, so the rounds of one Goal read back in order: what was asked, what was chosen,
what the owner overrode. That retained history is the progress view the Planning views idea
asked for, with no second feature to build.

**Prose stays the fallback, always.** The skill publishes only when it is running in a
session the manager owns and the publish succeeds. Outside the manager, against an older
manager, or when a surface is refused, it prints the round as prose exactly as it does
today. No CLI is required to support anything, which is what makes this hold across every
tool in `builtinTools`: publishing is a command, not a capability.

Every key has a click, every clickable surface has a key, and both are named in the footer
and under `?`.

## User stories

1. As the owner, I want a grilling round presented as a form, so that answering is choosing
   rather than retyping numbers.
2. As the owner, I want each question's recommended answer already chosen, so that agreeing
   with all of it costs one key.
3. As the owner, I want to override any single answer, so that agreement is per question
   and not all or nothing.
4. As the owner, I want a field for what the choices do not cover, so that an answer outside
   the options is still answerable.
5. As the owner, I want to send the whole round at once, so that the agent gets one coherent
   answer instead of a trickle.
6. As the owner, I want to leave a round unanswered and come back, so that opening it is not
   a commitment.
7. As the owner, I want the session's row to show that it is asking me something, so that I
   notice without watching its pane.
8. As the owner, I want the attention filter to keep sessions with an unanswered round, so
   that one key shows me everything waiting on me.
9. As the owner, I want nothing to open itself, so that a surface never lands on top of what
   I am typing somewhere else.
10. As the owner, I want my answer typed into the session only when its pane is at rest, so
    that it never lands on a dialog or over a half-typed line.
11. As the owner, I want the agent to read my answer as a normal turn, so that the
    conversation continues rather than branching.
12. As the owner, I want answered rounds kept, so that I can read what was asked and decided
    without scrolling a pane.
13. As the owner, I want to read a Goal's rounds back in order, so that its progress toward
    a Spec is legible after the fact.
14. As the owner, I want a malformed surface refused with a reason the agent can read, so
    that it fixes and republishes instead of failing silently.
15. As the owner, I want a surface naming a component this build cannot draw to still open,
    with that part skipped and labelled, so that a richer surface degrades rather than
    breaking.
16. As the owner, I want the skill to fall back to prose whenever it cannot publish, so that
    grilling outside the manager is unchanged.
17. As the owner, I want this to work whichever CLI the session runs, so that the feature is
    not a Claude Code feature.
18. As the owner, I want to publish and answer a surface on a session over SSH, so that a
    remote session is no less able to ask me something.
19. As the owner, I want every action in the surface reachable by key and by click, named in
    the footer and under `?`, so that it works the way the rest of the manager does.
20. As the owner, I want a session's surfaces to go when the session does, so that deleting
    work does not leave its questions behind.

## Implementation decisions

**`internal/ui/a2ui` owns the format.** A new child package holding the A2UI subset: the
document contract, its parsing, and rendering to strings. It is boundary-restricted the way
`internal/ui/runs` is, reaching no store, no tmux and no Git. Parsing is strict about the
version it reads and lenient about the rest: unknown fields are ignored, and a component
outside the subset is carried through as a skip with its type and a reason, which the
renderer paints as a labelled gap. That leniency is the whole reason the format rather than
a library is the contract — Fractory and the skills may write surfaces richer than this
build draws, and adopting `a2tea` later makes them appear without their authors changing
anything.

**The subset is a floor, stated once.** Text, card, column, row, list and divider for
layout; button, text field, check box and choice picker for interaction. A grilling round
needs exactly these. The floor is one list in one place, so adding to it is a visible,
reviewable change rather than a scatter of cases.

**`internal/ui/surfaces` owns the inbox.** A second child package for what the manager keeps
per session: which surface is current, which are answered, and the answer each carries. The
root adapter is `surfaces_adapter*.go` with the card as `surfaces_*_surface.go`. Both
packages are added to `tools/architecture/check-file-names`.

**The store grows one table, appended.** `agent_surfaces`, holding an id, the session it
belongs to, the published instant, the document as written, and the answered instant with
the answer when there is one. New schema is a new entry at the end of the migrations slice,
written so re-running it is harmless, and the rows go with the session when it is deleted.
A cap on retained surfaces per session keeps the store bounded; it is a named constant
expressed as how far back a reader would look, not a setting, because a value a user could
change here would be a second place for the same decision.

**One command layer, two fronts.** `internal/cli/surface.go` carries `surface publish` and
the reads a front needs, and the MCP tool mirrors it, kept in step by
`internal/cli/parity_test.go` as every other pair is. Publishing identifies the caller the
way `task` and `reserve` do, from the session whose pane ran it, and refuses a caller that
is not a session — including a terminal, which can be sent to but never asked.

**The answer reuses the inbox, and invents nothing.** Sending enqueues the answer on the
same `session_inbox` path a message between agents takes, so it inherits the at-rest gate,
the at-most-once delivery and the fenced formatting that already tell an agent where text
came from. The answer is fenced as the owner's answer to a published round, carrying the
surface's id so the agent can tell which round it belongs to.

**Store and tmux work goes on the effect lane**, as every other feature's does: the publish
read, the answer write and the retention trim are requests the root binds and applies, never
synchronous work on the update path.

**Nothing tool-specific, nothing platform-specific.** The feature is a command, a table and
a card. A session on an SSH connection publishes through the manager binary on its own host,
which is where its store and its pane are, and the surface is drawn by whichever manager the
owner is looking at.

## Testing decisions

A good test here asserts what the owner and the agent observe: the text a surface renders at
a width, the refusal a bad document earns, and the message the session receives when a round
is answered. None of it needs an agent, a network or a real grilling session.

**The first seam is `internal/ui/a2ui`**, driven as a package over fixture documents. It
carries the format's whole correctness: a round rendered to the text it should show, each
component of the subset, a component outside the subset skipped and labelled, a document of
another version refused, a malformed document refused with the field named, unknown fields
ignored, and a surface wide and narrow. Prior art is `internal/ui/automations`, whose
contracts are tested exactly this way.

**The second seam is the command layer**, over a temp store: a publish stores the document
against the calling session, a refusal names the problem, a caller that is not a session is
refused, the retention cap trims the oldest, and deleting a session takes its surfaces with
it. The parity test keeps the MCP front honest, which is why the layer rather than either
front is the seam. Prior art is `internal/cli/automations_test.go`.

**The third seam is the adapter, driven as a `Model`.** With a surface stored for a session:
the row carries the mark, the attention filter keeps that session, the key opens the card,
`esc` leaves it unanswered, and sending enqueues an answer carrying the surface id on the
session's inbox rather than writing to the pane directly. Prior art is
`internal/ui/automations_adapter_test.go` and the existing inbox tests.

Per this repo's rules the suite runs with `env -u TMUX TMUX_TMPDIR=/tmp/amtest` and
`SHELL=/bin/sh`. Because this is a change to the UI and to delivery, it is verified against
real sessions before the pull request says it works: build the binary, run it under a
throwaway `HOME` on its own socket, publish a surface from inside a real session with the
CLI, answer it, and read the frames and the receiving pane back. A green suite is not that.

## Open questions

- Whether a Run that stops for a human should move onto this path, replacing its text field
  with a published surface. The obvious next use, and a change to Fractory as well.
- Whether the skill publishes one surface per round or one per question. Per round is what
  this Spec assumes, since a round is the unit the owner answers.
- Whether retained rounds deserve a reader of their own, rather than being read through the
  session they belong to.
- Whether a surface should survive its session being archived, which keeps the history but
  leaves questions nobody can answer.
