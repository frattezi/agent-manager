# Spec 0004: Runs rail rows read as work, not as references

Status: accepted (2026-11-27). Vocabulary: [GLOSSARY.md](../../GLOSSARY.md).

## Goal

The Runs section of the rail is where the owner looks to see what the factory is doing. Today
each row is named by the Ticket Ref the ticket source gave it — `github:frattezi/esquerda-nas-ruas#270`
— so the first twenty columns of every row carry the same two words. On a rail that is thirty
columns wide the row has nothing left to say which Run it is. Rows become distinguishable only
by the status word at their right edge, and a reader scanning eight Runs across four Projects
cannot tell any of them apart.

The owner wants a row to read as the work it is: which Project it belongs to, what it is doing,
and the one number worth clicking. The Ticket Ref stays the Run's identity everywhere identity
matters; it stops being what a human is shown.

## Non-goals

- The `fractory runs` CLI table, which repeats the same constant prefix. It lives in
  `frattezi/fractory` and is filed there.
- Having Fractory record a Run Title on the Run record. Also filed in `frattezi/fractory`; this
  Spec prefers that field the moment it exists and works without it until then
  ([ADR 0005](../adr/0005-run-titles-recovered-from-branch-slugs.md)).
- Repairing what the branch slug lost. An apostrophe that reached the branch as `-s-` comes back
  as ` s `, and a slug Fractory truncated mid-word stays truncated.
- The Runs section header, the fold, the finished-Runs toggle, the re-run and solved marks, and
  the Automations surface, which is a different thing.
- Reading a Run's session, PR state or result summary, all of which sit unread on the record and
  are not needed here.

## Solution

A Run row reads, left to right: the status glyph, the Project, the Run Title, then at the right
edge the Mode badge, a number, and the status word.

**Project** is the name of the repository the Run's work belongs to, taken from the Run's
checkout directory rather than parsed out of the Ticket Ref, so a Run whose Ticket lives in Jira
is named as readily as one whose Ticket lives on GitHub. It is shown as a dimmed prefix and is
the first thing dropped when the rail is narrow, because the owner usually knows which Project
they are looking at and does not know which Run.

**Run Title** is the human sentence. Where the record carries a Title, that is used. Where it
does not, the Title is recovered from the Run's branch: everything up to the last `/` is
dropped, a leading ticket number is dropped, hyphens become spaces, and the first letter is
capitalised. `fractory/270-wire-marina-s-logo-into-the-favicon-foot` reads as
`Wire marina s logo into the favicon foot`. It is the row's main text and the last thing to go:
it truncates with an ellipsis rather than disappearing.

**The number** is the one the owner would click. Once a Run has a pull request, the row shows
that pull request's number; until then it shows the Ticket's number. The two are told apart on
the row, because they differ — Ticket 270 produced pull request 293 — and a row that showed a
bare `293` would be ambiguous. The slot is never empty: every Run has a Ticket number, and
roughly a third of them never get a pull request.

When the rail is too narrow for all of it, the row gives up the Project first, then the number,
and only then truncates the Title. The status glyph, the Mode badge and the status word always
survive: the glyph and the word are how the owner reads state at a glance, and the badge is one
column that says whether a human is in the loop.

Nothing is lost by a row that dropped something. The detail pane beside the rail lists the
Project and the Title alongside the full Ticket Ref, the branch, the pull request URL, the
worktree and the log. Search still matches the full Ticket Ref and the branch, so every search
the owner runs today keeps working, and now also matches the Project and the Title. Opening a
Run's diff still names its Review target by the full Ticket Ref, because a review target wants
to be unambiguous across Projects more than it wants to be readable. The row's own menu is
titled by the Run Title, because a menu title is something a human reads.

## User stories

1. As the owner, I want a Run row to show the Run Title rather than the Ticket Ref, so that I
   can tell two Runs apart without reading their branches.
2. As the owner, I want the forge and my own username off the row, so that no column is spent on
   something that is the same for every Run I have.
3. As the owner, I want the Project on the row when the rail is wide enough, so that I can see
   which repository a Run belongs to while Runs from several Projects are interleaved.
4. As the owner, I want the Project named from the Run's checkout, so that a Run whose Ticket
   comes from Jira is named as well as one whose Ticket comes from GitHub.
5. As the owner, I want a Run with no Title on its record to still show one, so that the rail is
   readable before Fractory records titles.
6. As the owner, I want a Title that Fractory recorded to be preferred over the one recovered
   from the branch, so that the rail improves on its own when that field arrives.
7. As the owner, I want the pull request number on a Run that has a pull request, so that I can
   go straight to the thing I need to review.
8. As the owner, I want the Ticket number on a Run that has no pull request yet, so that the row
   never shows an empty slot.
9. As the owner, I want to see at a glance whether a number is a Ticket or a pull request, so
   that I do not open the wrong one.
10. As the owner, I want the Title to be the last thing the row gives up, so that a narrow rail
    still tells me what each Run is doing.
11. As the owner, I want the status glyph, the Mode badge and the status word to survive any
    width, so that state and oversight level are never the thing that got truncated.
12. As the owner, I want rows to stay aligned when a Mode badge or a re-run marker is present,
    so that the right-hand column does not wander.
13. As the owner, I want the detail pane to show the Project and the Title next to the full
    Ticket Ref, so that anything the row dropped is one keystroke away.
14. As the owner, I want searching for a Ticket Ref, a number or a branch to keep working, so
    that no search I have learned stops finding its Run.
15. As the owner, I want searching for a Project name or words from a Title to find the Run, so
    that I can search for what I can now see.
16. As the owner, I want a Run's row menu titled by its Run Title, so that the menu names the
    work the way the row does.
17. As the owner, I want a Review target still named by the full Ticket Ref, so that a diff I
    opened is unambiguous when several Projects are in play.
18. As the owner, I want every one of these to hold on a thirty-column rail and on a wide one,
    so that the feature is not only true at the width it was written at.

## Implementation decisions

**The derivations are pure functions on the Run record, not on the view.** `internal/ui/runs`
gains three derivations over a `Record`: the Project, the Run Title, and the number with its
kind. They take a record and return strings; they read no files, no network and no clock. The
rail is a renderer and learns none of this.

- Project: the base name of the record's checkout path. Empty when the path is empty.
- Run Title: the record's own Title when non-empty, else the branch de-slugged by the four steps
  in the Solution. Empty when the branch is empty, in which case the row falls back to the
  Ticket Ref so a row is never nameless.
- Number: a value carrying the digits and whether they came from a pull request or a Ticket. The
  pull request number is the last path segment of the record's pull request URL, used only when
  that segment is all digits. The Ticket number is the digits after the last `#` in the Ticket
  Ref, and when the Ref has no such digits the number is absent and the slot renders empty
  rather than guessing from the branch.

**The record grows one optional field.** The Run record contract gains an optional title written
by Fractory. Absent in every record on disk today, which is why the de-slug exists. Adding an
optional field does not move `RecordVersion`; the manager already ignores fields it does not
decode, and a record without the field must keep parsing.

**The rail row carries finished strings.** The rail's `Run` view struct gains the Project, the
Title and the rendered number, computed by the adapter when it maps a record to a row — the same
shape as the Mode badge, which is already passed in as a string. The struct keeps its Ticket Ref
field, which the menu, the search text and the Review target still need.

**The width budget measures what is painted.** The row's budget for its name today subtracts the
bare status word while the right-hand column paints the Mode badge and the status word, and the
re-run marker has already been appended to the glyph on the left. Both are unbudgeted, so a
badge or a marker overflows the row before this Spec adds anything. The budget is recomputed
from the strings that are actually painted, and the drop order is applied against that budget:
drop the Project if the Title would get less than its floor, then drop the number, then truncate
the Title.

**The Project appears by a room rule, not a terminal width.** The Project is included only when
the Title still keeps a floor of columns after it. One named constant for that floor, expressed
as what a reader needs rather than as a terminal size, because a width threshold frozen in code
is a value that stops receiving fixes the day it is written.

**Keyboard and mouse are unaffected.** No row, handle or clickable surface is added or removed;
Run rows remain read-only and keep having no drag handle.

## Testing decisions

A good test here asserts what a reader sees: the text of a rendered row at a chosen width. It
never asserts a format string, a column position or an intermediate struct, because the point of
the change is the reading and the drop order is only observable in a rendered frame.

Two seams, the highest available for each half.

**The derivations, as a table test in `internal/ui/runs`.** A record in, three strings out. This
is where the de-slug's cases are pinned: a branch with a ticket number and a prefix, a branch
with neither, an apostrophe that arrived as `-s-`, a slug truncated mid-word, an empty branch, a
record carrying a Title that must win over the branch, a pull request URL whose last segment is
and is not digits, a Ticket Ref with no number, and a Jira Ref. Prior art: the record parsing
tests already in that package.

**The row, through the rail's render seam.** `rail.New`, `Reconcile` a snapshot of Runs, `Render`
at a width, strip the escapes, assert on the text — exactly as `internal/ui/rail/runs_test.go`
already does for the Ticket and the Mode badge. This seam carries the whole of the Solution:
that the forge prefix is gone, that the Title is there, that the Project appears at a wide width
and not at a narrow one, that the number shows a pull request when there is one and a Ticket
when there is not, that the drop order is Project then number then Title, and that the glyph,
badge and status word survive thirty columns. Width is an argument to the render context, so the
narrow and wide cases are the same test driven twice.

The adapter's mapping from record to row needs no seam of its own: it is covered from both sides
once the derivations and the rendering are pinned.

Search, the detail pane and the menu title are asserted where their existing tests live — the
search test in the rail package, the detail pane in the Runs adapter's test — extended rather
than replaced.

A change to the rail is verified against real sessions before it is called done: build the
binary, run it under a throwaway `HOME` on its own socket, and read the frames back with real
Runs in the registry. The suite is not that.

## Open questions

- Whether the recovered Title should drop a trailing fragment left by Fractory's mid-word
  truncation. Not doing it, per ADR 0005: any rule that trims a fragment also trims the last
  word of a title that happened to end on a short one. Worth revisiting only if the field
  Fractory will record does not arrive.
- Whether the detail pane should also surface the result summary and pull request state that sit
  unread on the record. Independent of this Spec, and a change to the pane rather than the row.
