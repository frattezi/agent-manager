# Spec 0006: Tickets are visible before their first Run

Status: accepted (2026-11-27). Vocabulary: [GLOSSARY.md](../../GLOSSARY.md).

## Goal

The owner's work exists before the factory touches it. A Goal is grilled into a Spec, the
Spec becomes Tickets, and those Tickets wait — in the backlog, held on a blocker, or ready
for the next tick. None of that is visible in Fractory Manager. The rail's section of
factory work is built from Run records, so a Ticket appears only once a Run exists for it,
and even then the row carries a Ticket Ref rather than anything a human would recognise.
To see what is queued, or why nothing started, the owner leaves the manager and opens
GitHub.

The owner wants the whole board where they already are: every Ticket of every Project the
factory manages, with its title, its status, the Spec it came from, and the Runs and pull
requests on it. Reading, not driving — the factory still raises Runs and moves statuses.

## Non-goals

- **Creating a Ticket or changing its status.** `to-tickets` creates Tickets from an
  approved Spec, and Fractory's tick moves them through `in_progress`, `in_review` and
  `done`. This Spec adds no write path to the ticket source.
- **The manager calling the forge.** Refused by
  [ADR 0005](../adr/0005-run-titles-recovered-from-branch-slugs.md): a network call behind
  every row breaks the rail offline and binds the manager to one forge's API. The manager
  reads files, as it does for Runs and Automations.
- **The writer of the Ticket Snapshot.** Fractory fetches Tickets and writes the snapshot;
  that is a sibling Spec in `frattezi/fractory`. This Spec defines the contract and
  behaves without it.
- **Parsing a Ticket body.** The `## Spec` heading is a convention of the body that
  Fractory already holds in memory; it resolves the Spec path and records it. The manager
  never parses forge prose.
- **A new base screen.** The Tickets live in the rail section that already lists them,
  beside the sessions, with the detail column carrying what a row cannot.
- **Making a row readable at thirty columns.** That is
  [Spec 0004](0004-runs-rail-rows-read-as-work.md), a co-requisite rather than a
  dependency: this Spec adds rows to a section whose rendering that Spec fixes, and
  neither waits for the other.
- **The Grilling view.** The other half of the Planning views idea is its own Spec, on
  Agent Surfaces ([ADR 0007](../adr/0007-agent-authored-surfaces-are-a2ui.md)).
- **Tickets of repositories the factory does not manage.** A Ticket with no Project is not
  work the factory can raise a Run for.

## Solution

The section the rail already draws for factory work is renamed to say what its rows are:
**Tickets**. It lists every Ticket the factory knows of, not only the ones that reached a
Run, and each row reads as the piece of work it is.

**Where a Ticket comes from.** Two sources are merged, keyed by Ticket Ref. The Ticket
Snapshot, which Fractory writes per Project, carries the title, the status, the URL, the
blockers, the parent and the Spec path. The Run registry carries every Run on that Ticket.
A Ticket named by either appears; a Ticket named by both is one row. A Project whose
snapshot has never been written still shows the Tickets its Runs name, exactly as the
section does today, and says that its backlog is unknown rather than implying it is empty.

**Status.** A Ticket has a status in the ticket source and its latest Run has a status of
its own, and they disagree by design: a Run that stopped for a human leaves the Ticket at
`in_progress`. The row shows the **Run's** state whenever a Run is outstanding — queued,
running, needs-human, failed — because that is what the owner must act on, and falls back
to the Ticket's own status otherwise. A Ticket held on a blocker is shown as held, read
from the host state the manager already polls. No state is ever hidden behind a milder one.

**Grouping.** Tickets are grouped under their Project, which folds. The Project rows appear
only when more than one Project has a Ticket to list, so a single-Project workspace is not
given a row that says nothing. A Project is named from the Run's or the snapshot's checkout
directory, per Spec 0004, so a Jira-backed Project is named as readily as a GitHub one.

**Reading one Ticket.** The detail column beside the rail carries the whole of it: the
title, the Project, the status and where it came from, the Spec path, the blockers and the
parent, then every Run with its branch, its pull request and its log, and when the snapshot
that fed the row was taken.

**Acting on what a row points at.** A Ticket with a Run keeps everything the section does
today: its log, a re-run, an answer to a Run that stopped, and the solved mark. Three
openings are added, each on a Ticket with the thing to open: the Spec in the editor, the
Ticket in the forge, and the pull request in the forge. A Ticket with no Run says so rather
than opening an empty log.

**Staleness is shown, never hidden.** A snapshot is a reading taken at an instant, and the
owner is told its age in the detail column, with the row saying so once the reading is old
enough that acting on it would be a mistake. The manager cannot refresh it: it has no forge
credentials and no business holding any. A stale board is Fractory's to refresh on its next
sweep.

Every key added has a click, every click a key, and both appear in the footer and under `?`.

## User stories

1. As the owner, I want to see a Ticket before any Run exists for it, so that the work I
   planned this morning is visible this afternoon.
2. As the owner, I want a Ticket's title on its row, so that I recognise the work without
   decoding a Ticket Ref.
3. As the owner, I want a Ticket's status on its row, so that I can tell the backlog from
   what is ready from what is in review.
4. As the owner, I want a Run's state to win over the Ticket's status while a Run is
   outstanding, so that a Run waiting on me is never hidden behind `in_progress`.
5. As the owner, I want a Ticket held on a blocker shown as held, so that I understand why
   it has not started instead of assuming the factory is idle.
6. As the owner, I want every Ticket of every Project the factory manages, so that one
   board answers what is happening across my repositories.
7. As the owner, I want Tickets grouped under their Project, so that interleaved work from
   several repositories is readable.
8. As the owner, I want the Project rows left out when only one Project has Tickets, so
   that a single-repository workspace is not padded with a row that says nothing.
9. As the owner, I want a Project group to fold, so that I can collapse the repository I am
   not working on.
10. As the owner, I want the Spec path on a Ticket, so that I can see which Spec produced
    the work.
11. As the owner, I want to open that Spec in my editor from the row, so that reading the
    intent behind a Ticket is one key away.
12. As the owner, I want to open a Ticket in the forge from the row, so that commenting or
    editing it does not mean searching for it.
13. As the owner, I want to open a Run's pull request from the row, so that I can go
    straight to the review.
14. As the owner, I want every Run of a Ticket in the detail column, so that I can see a
    Ticket's history rather than only its latest attempt.
15. As the owner, I want the log, the re-run, the answer and the solved mark to keep
    working exactly as they do today, so that nothing I rely on is traded for the backlog.
16. As the owner, I want a Ticket with no Run to tell me so, so that a key that cannot work
    explains itself instead of opening an empty tab.
17. As the owner, I want finished Tickets hidden behind the key that hides them today, so
    that a long history does not bury what is live.
18. As the owner, I want the attention filter to keep the Tickets that need me — a Run that
    failed, a Run that stopped for a human, a Ticket held on a blocker — so that one key
    narrows the board to my queue.
19. As the owner, I want search to match a Ticket's title, its ref, its status and its
    Project, so that I can find work by whatever I remember of it.
20. As the owner, I want to see when the board was last read from the ticket source, so
    that I know how much to trust it.
21. As the owner, I want a Project whose snapshot was never written to show its Runs and
    say its backlog is unknown, so that an absent snapshot is never read as an empty
    backlog.
22. As the owner, I want the manager to work with no Ticket Snapshot at all, so that
    installing this release before Fractory writes one changes nothing for the worse.
23. As the owner, I want a snapshot from a newer Fractory to be refused rather than
    half-read, so that a contract change cannot show me wrong work.
24. As the owner, I want every Ticket action reachable by key and by click, named in the
    footer and under `?`, so that the section works the way the rest of the manager does.
25. As the owner, I want all of it to hold on a thirty-column rail and on a wide one, so
    that the board is not only true at the width it was written at.

## Implementation decisions

**A new child package, `internal/ui/tickets`.** It owns the Ticket Snapshot contract, its
parsing, and the merge of snapshot, Run records and held state into the board the view
lists. It is boundary-restricted the way `internal/ui/runs` is, reaching nothing but the
file system and `internal/atomicfile`. The dependency runs one way: `tickets` reads the
records `runs` parses and never the reverse, so the Run registry keeps its own contract and
its own tests. The root adapter is `tickets_adapter*.go`, and `tickets` is added to
`tools/architecture/check-file-names`.

**Ticket Snapshot, version 1.** One document per managed Project, written by Fractory into
`$FRACTORY_HOME/tickets/`. Its file name is opaque to the manager, which reads every
`.json` in the directory and keys on the content, as the Automation Run reader does:

- `version`, pinned to 1
- `repoPath`, the absolute checkout the Project is named from
- `source`, the kind of ticket source the reading came from
- `takenAt`, the instant Fractory read the ticket source
- `tickets`, each carrying `ref`, `title`, `url`, `status`
  (`backlog` | `ready` | `in_progress` | `in_review` | `done`), `updatedAt`, `blockedBy`,
  an optional `parent`, and an optional `specPath` Fractory resolved from the Ticket body

A document whose version is not 1 is skipped with a reason, as is one missing `repoPath`,
`takenAt` or a Ticket's `ref` or `status`. Unknown fields are ignored, so Fractory can
extend the contract without a manager release. Fractory owns the schema in its
`src/contracts/`; the Go side mirrors it pinned to the version it reads — the arrangement
[ADR 0004](../adr/0004-automations-configured-here-executed-by-fractory.md) already
established for Automations.

**The board is a pure function.** Snapshot documents, Run records, host state and `now` go
in; the ordered, grouped list of Tickets comes out, each with its status, its Project, its
Spec path, its Runs and the age of the reading behind it. It reads no files and no clock of
its own, which is what makes the whole of the Solution testable as data.

**Status precedence is a rule, not a rendering.** The derived status of a Ticket is its
latest Run's status while that Run is outstanding or ended needing a human or failing,
otherwise the Ticket's snapshot status, otherwise — with no snapshot — what the section
derives from Run records today. Held is carried alongside rather than replacing it, since
a Ticket is held in addition to being ready.

**The section's row kinds are renamed to say what they carry**, and a Project row is added
above them. Renaming changes the selection keys the saved folds are stored under, so folds
saved by an earlier release no longer match: they are dropped silently, because a fold is
cosmetic state and a wrong fold is worse than a lost one.

**The snapshot is read on the poll that already reads the registry.** The section's poll is
a plain command off the update path, not the effect lane, because it is file IO with no
store, tmux or Git work — the shape `internal/ui/runs` uses. One read gathers the records,
the snapshots and the host state together, and a read whose result equals the current state
rebuilds no rows.

**No refresh key and no setting.** The manager holds no forge credentials and must not, so
there is nothing for a refresh key to do; the sweep belongs to Fractory. The age at which a
reading is called old is a named constant expressed as what a reader needs, not a knob: a
value a user could set here would be a second place for the same decision, and the exact
age is always in the detail column regardless.

**Nothing tool-specific and nothing platform-specific.** The board is files on disk and a
rail section, so it holds for every CLI in `builtinTools` without an adapter touching it.
Opening a Ticket or a pull request goes through the link opener the manager already uses,
which is where the platform difference already lives; opening a Spec goes through the
existing editor path.

## Testing decisions

A good test here asserts what the owner ends up looking at: which Tickets the board holds,
in what order, under which Project, with which status, and what a row says at a given
width. None of it needs a forge, a network, Docker or Fractory.

**The seam to aim for is `internal/ui/tickets`**, exercised as a package over a temp
directory with fixture snapshot documents, fixture Run records, a fixture host state and an
injected `now`. It carries the whole of the Solution's correctness: the union keyed by
Ticket Ref, the status precedence including the needs-human and held cases, the Project
grouping and the single-Project rule, the ordering, the staleness threshold, a Project with
Runs and no snapshot, no snapshot directory at all, a document of another version, a
document missing a required field, and unknown fields ignored. Prior art is
`internal/ui/automations`, whose records, requests and due computation are tested exactly
this way, and `internal/ui/runs`'s registry tests.

Two supporting seams, both with prior art, and neither repeating the first:

**The rows, through the rail's render seam.** `rail.New`, `Reconcile` a snapshot of the
board, `Render` at a width, strip the escapes and assert the text — as
`internal/ui/rail/runs_test.go` already does for the Ticket and the Mode badge. This pins
the rename, the Project rows appearing and not appearing, the fold, and that a title
survives thirty columns while the status word and the glyph do.

**The adapter, driven as a `Model`.** Fixture snapshots and records in, then assert the
detail column's contents, the legend, that the Spec opens in the editor and the Ticket and
pull request through the link opener, and that a Ticket with no Run refuses with a reason —
as `internal/ui/runs_adapter_test.go` drives the section today. The merge is not re-asserted
here; it is covered by the package seam.

Search, the attention filter and the finished toggle are extended where their tests already
live in the rail package rather than given new ones.

Per this repo's rules the suite runs with `env -u TMUX TMUX_TMPDIR=/tmp/amtest` and
`SHELL=/bin/sh`. A change to the rail and the poller is verified against real sessions
before the pull request says it works: build the binary, run it under a throwaway `HOME` on
its own socket, put real snapshot documents and real Runs in front of it, and read the
frames back at a narrow width and a wide one. A green suite is not that.

## Open questions

- Whether the manager may ask Fractory to refresh a stale reading by dropping a request
  file, the way Run re-runs and Automations already ask for work. It would make a stale
  board actionable without giving the manager credentials, and it is a new contract for a
  problem that may not bite.
- Whether a `done` Ticket that never had a Run should appear when finished Tickets are
  shown, or stay out: it is history the factory never touched.
- Whether the Ticket's number belongs in the snapshot or stays derived from the Ticket Ref,
  which Spec 0004 already derives it from for the row.
- Whether `specPath` resolution is reliable for a Jira body, whose rich text Fractory
  flattens before it can look for the heading. Fractory's side of the contract, raised
  there.
