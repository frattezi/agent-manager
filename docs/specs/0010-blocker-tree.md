# Spec 0010: A Blocker Tree in the Runs Overview

Status: accepted (2026-11-28), amended 2026-11-28: the tree shows every unfinished Ticket, goes
first, and shows a running Ticket's progress. Vocabulary: [GLOSSARY.md](../../GLOSSARY.md).

## Goal

A Spec is split into Tickets that wait on each other, and the factory starts, stacks and holds
them by their blockers. None of that shape is visible in Fractory Manager: the Runs section lists
Tickets one row at a time, and the only place that says why #85 has not started is
`fractory status`. The owner wants to see, per Project, which Tickets wait on which, how far each
one has got, and where the tree is stuck.

## Non-goals

- Reading the tracker. Blockers come from what Fractory records; a Ticket that has no Run and is
  not held is only known as someone's blocker.
- Changing blockers from the manager. The tree is read-only, like the rest of the Overview.
- The Ticket Snapshot ([Spec 0006](0006-tickets-visible-before-their-first-run.md)). If it lands,
  it can become a better source; this Spec does not wait for it.
- Blockers across Projects. Fractory holds such a Ticket until the owner fixes the line, so the
  tree never needs to draw one.

## Solution

Each Project in the Runs Overview ([Spec 0009](0009-section-overviews.md)) opens with a
**Blocker Tree** block, right under its header and In flight line and above the charts: live work
first, history below. Every unfinished Ticket is in it. A Ticket with no blocker and nothing
waiting on it is a root of its own, so every running, held or waiting Ticket is on screen.

The tree reads top-down: a Ticket with no blocker is a root, and each Ticket sits under every
Ticket it waits on. Lines are drawn with box-drawing characters (`├─`, `└─`, `│`), and with
plain ASCII under the Compatible glyph mode. A Ticket with two blockers appears under each; its
second appearance is dimmed and marked as a join, so the eye follows one copy.

Each node shows the status glyph and colour of its latest Run, its Run Title, and its pull
request or Ticket number as the Runs rows show them. A held Ticket also shows why each blocker
holds it, in Fractory's words ("not done: its latest Run failed"). A blocker that has no Run and
is not held is drawn dimmed, as its Ticket number with "no Run yet".

A running Ticket carries two more lines: how long its Run has run against the Project's usual
Run time, as a bar and in words ("13m of a usual 22m"), and the last line of the Run's log, as
what it is doing now. The usual Run time is the median duration of the Project's finished Runs in
the window; with none, the bar is left out and the time stands alone. A needs-human Ticket shows
the first line of its question, dimmed.

A tree stays in the Overview while any Ticket in it is unfinished. Once every Ticket in it is
done, the time window decides, by its newest Run, as it does for everything else in the Runs
Overview.

## User stories

1. As the owner, I want each Project's Blocker Tree in the Runs Overview, so that I see how its
   Tickets wait on each other without leaving the manager.
2. As the owner, I want each Ticket drawn under the Tickets it waits on, so that the order the
   factory will build them in is the order I read.
3. As the owner, I want each node's status, Run Title and number, so that I see how far each
   Ticket has got.
4. As the owner, I want a held Ticket to say why its blocker holds it, so that I see where the
   tree is stuck and what would unstick it.
5. As the owner, I want a Ticket with two blockers under both, its second copy dimmed, so that
   I find it from either side without counting it twice.
6. As the owner, I want a blocker with no Run drawn dimmed with "no Run yet", so that a tree
   waiting on backlog work says so.
7. As the owner, I want every unfinished Ticket in the tree, a lone one as its own root, so
   that every Run in progress is on screen and not only a count.
8. As the owner, I want an unfinished tree kept whatever the window, so that live work never
   falls out of view.
9. As the owner, I want the tree drawn in ASCII under the Compatible glyph mode, so that any
   terminal draws it.
10. As the owner, I want the tree to scroll with the rest of the Overview, so that a deep tree is
    never cut off.
11. As the owner, I want the tree first in each Project, above the charts, so that live work is
    what I see before history.
12. As the owner, I want a running Ticket to show its time against the usual Run time and its
    log's last line, so that I know how far along it is and what it is doing.
13. As the owner, I want a needs-human Ticket to show the first line of its question, so that I
    see what it waits on without opening it.

## Implementation decisions

**Fractory records blockers on the Run.** The Run record gains an optional `blockedBy`: the
Ticket Refs of the Ticket's blockers, read from its Relations section when the Run is queued.
Optional, so `RecordVersion` does not move and older records parse with none. This is a change
in `frattezi/fractory`, filed there; this Spec is blocked by it.

**Edges.** A Ticket's blockers are its latest Run's `blockedBy`. While the Ticket is held, the
blockers Fractory lists for it in its host state win, being newer. The manager decodes the held
Tickets in the host state exactly as Fractory's `src/contracts/host-state.ts` writes them: each
has `repoPath`, `ticketRef`, `blockers` (refs) and optional `holds` (`blocker` and `reason`). The
Project is matched by `repoPath`; a blocker with no `holds` entry has no reason drawn.

**The tree is computed in the overview package.** A pure function over Run records, held
Tickets, the window and the clock returns the trees per Project: roots, children in Ticket
order, join markers, and the hold reasons. Blockers that form a cycle are drawn once, as roots,
each marked as part of a cycle, since Fractory holds them all. A blocker known only by its Ref
becomes a node with no Run.

**Progress.** The log's last line is read only for running Runs: the last non-empty line of
`logPath`, read from the end of the file on the poll that already reads the registry, never in
`Update`, with escapes stripped and cut to one line. The usual Run time comes from the same
records the Run time sparkline uses.

**Drawing.** The tree is drawn by the overview package next to the other blocks, with the
Project's colour on the roots and the theme's status colours on nodes. It reuses the Runs rows'
Run Title and number derivations from Spec 0004.

## Testing decisions

The same two seams as Spec 0009. The tree computation gets table tests: a single chain, a
Ticket with two blockers, a blocker with no Run, a held Ticket whose host-state blockers differ
from its last Run's, a cycle, a finished tree outside the window and an unfinished one kept. The
drawing is asserted through the content pane at a narrow and a wide width with escapes
stripped: lines, joins, hold reasons, the dimmed "no Run yet" node, and no box-drawing runes
under the Compatible glyph mode. Decoding `blockedBy` and the held Tickets is asserted where
record and host state parsing are tested today, including records without them.

Verified against real Runs before it is called done.

## Open questions

- How deep a tree stays readable at a 30-column pane. The real-session check decides whether
  deep levels need to collapse.
