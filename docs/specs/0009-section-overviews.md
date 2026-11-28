# Spec 0009: Overviews for the Runs, Automations and Bots sections

Status: accepted (2026-11-27). Vocabulary: [GLOSSARY.md](../../GLOSSARY.md).

## Goal

The rail's Runs, Automations and Bots sections each list their things one row at a time, and
the content pane beside them only ever describes one row. Selecting a section's header row
shows almost nothing: a held-queue line for Runs, one sentence for Automations, two counts for
Bots. To know how a Project is doing — what is in flight, how much the factory finished, whether
its pull requests land, what the work was — the owner reads rows one by one, or leaves the
manager.

The owner wants each section header to open an **Overview**: a read-only, good-looking summary
of that section's things, in colour and with charts, built from what is already recorded on
disk. Runs and Automations are summarised per Project; Bots by Profile, live. Nothing in an
Overview is written by an agent or by the view: what was done is told by the Run that did it.

## Non-goals

- Tokens, cost and model usage. They are not on the Run record, and computing them means
  reading CLI transcripts; deferred until Fractory records them.
- Bot history. No history of a Bot is recorded, and recording one is a feature, not an Overview.
- Projects with no Run on record. The manager does not read Fractory's list of repositories.
- Navigation from an Overview: no Project line filters, jumps or opens anything. The rows below
  the header stay where the owner acts.
- Tickets that have never had a Run ([Spec 0006](0006-tickets-visible-before-their-first-run.md)).
- The Run row itself: how a Run row names its Project and Run Title is draft Spec 0004 (Runs
  rail rows read as work), which lands first and whose derivations this Spec reuses.
- Fixing where the manager reads and writes Automation records. The manager reads
  `automation-runs/` and writes requests to `automations/`, while Fractory writes
  `automations/<id>/` and reads `automations/requests/`; that is a separate fix that lands
  before the Automations slice, because the Automations Overview has nothing to draw without it.

## Solution

**Where.** Selecting the Runs, Automations or Bots header row fills the content pane with that
section's Overview. Selecting any row under the header shows that row's detail as today. Any
existing header content (the held-queue line on Runs) stays at the top of the Overview.

**How it looks.** Charts are drawn with ntcharts and composed with lipgloss: bordered blocks,
aligned columns, coloured strips, stacked bars and sparklines. There is no fallback to bare
numbers. Under the plain glyph setting, charts still draw, with block and ASCII runes instead of
braille. Status colours are the theme's own (working, waiting, finished, errored, idle). Each
Project has a stable colour taken from the theme's swatches, the same set a Bot Profile picks
from, so every theme — light, dark, custom — holds.

**Time window.** The Runs and Automations Overviews share one window, which steps through
24h, 7d, 30d and all. The manager starts on all, and the window is forgotten when it quits. One
key steps it; clicking the window label at the top of the Overview steps it too; the footer and
the `?` key map name the key. "In flight" is always now, whatever the window.

**Scrolling.** An Overview taller than the pane scrolls, by the mouse wheel and by keys, the
same keys the footer names. Nothing is silently cut off.

**Runs Overview.** One block per Project with at least one Run in the window, the Project with
the most recent activity first. Each block shows:

1. A header: the Project name in its colour, its Mode badge, its base branch, and its gates
   (Ticket Gate, PR Gate, auto-merge).
2. In flight: a coloured strip of its queued, running and needs-human Runs, with counts.
3. Throughput: Runs finished per period across the window, as bars stacked by outcome (done,
   failed, needs-human).
4. The pull request funnel, counted in Tickets by their latest Run: Tickets, pull requests
   opened, merged, closed unmerged.
5. A sparkline of Run durations, created to finished, oldest to newest.
6. The latest three finished Runs, each as its Run Title over its result summary.

**Automations Overview.** Automations grouped by the Project their work runs in, in the same
Project colours, with Automations outside any Project under "Elsewhere". Each Automation shows
its upcoming fires on a next-24-hours timeline, its last ten firings as outcome-coloured blocks,
and its latest firing's summary. A disabled Automation is shown dimmed and has no upcoming fires.

**Bots Overview.** Each Bot Profile in its own colour and emoji, with its live Bots as
status-coloured dots and what each is doing. A Profile with no live Bot is dimmed. The window
does not apply.

**Empty states.** A section with nothing to summarise says so in one line, and for Bots keeps
today's hint on how to create a Profile.

## User stories

1. As the owner, I want selecting the Runs header to show an Overview of the factory's work, so
   that I see every Project at once without reading Run rows one by one.
2. As the owner, I want the Overview grouped by Project, so that work from several repositories
   is not interleaved.
3. As the owner, I want the most recently active Project first, so that what I am working on is
   at the top.
4. As the owner, I want each Project block to open with its name, Mode, base branch and gates,
   so that I know how much oversight that Project's work gets.
5. As the owner, I want each Project's queued, running and needs-human Runs as a coloured strip,
   so that I see at a glance where the factory is busy and where it waits on me.
6. As the owner, I want finished Runs per period as stacked bars by outcome, so that I see how
   much the factory got done and how much of it failed.
7. As the owner, I want a pull request funnel per Project counted in Tickets, so that I see how
   much of the work lands, without one pull request counted once per Run on it.
8. As the owner, I want a sparkline of Run durations per Project, so that I notice when Runs get
   slower.
9. As the owner, I want the latest three finished Runs with their Titles and summaries, so that I
   know what was actually done.
10. As the owner, I want every summary to come from the Run that did the work, so that the
    Overview never makes up what happened.
11. As the owner, I want selecting the Automations header to show an Overview grouped by
    Project, so that it reads like the Runs Overview.
12. As the owner, I want each Automation's upcoming fires on a 24-hour timeline, so that I see
    what will run and when.
13. As the owner, I want each Automation's last ten firings as coloured blocks, so that a
    failing Automation stands out.
14. As the owner, I want each Automation's latest summary, so that I know what it last did.
15. As the owner, I want Automations that run outside any Project listed under Elsewhere, so
    that none is missing.
16. As the owner, I want selecting the Bots header to show each Profile with its live Bots, so
    that I see which Bots are working and which wait on me.
17. As the owner, I want a Profile with no live Bot dimmed, so that what is running stands out.
18. As the owner, I want one time window shared by the Runs and Automations Overviews, stepping
    24h, 7d, 30d and all, so that I can look at today or at everything.
19. As the owner, I want the window to start at all every time the manager starts, so that I am
    never looking at a narrow window I forgot I chose.
20. As the owner, I want to step the window with one key or a click on its label, so that the
    keyboard and the mouse both reach it, and the footer and `?` to name the key.
21. As the owner, I want a tall Overview to scroll with the wheel and with keys, so that nothing
    is cut off.
22. As the owner, I want charts in colour from my theme, so that the Overview fits whatever
    theme I picked.
23. As the owner, I want each Project to keep the same colour across Overviews and restarts, so
    that I learn to recognise it.
24. As the owner, I want charts to still draw under the plain glyph setting, with block or ASCII
    runes, so that a terminal without braille shows a chart and not boxes.
25. As the owner, I want an empty section to say so in one line, so that an empty pane never
    looks broken.
26. As the owner, I want selecting a row under a header to keep showing that row's detail, so
    that nothing I use today moves.
27. As the owner, I want the Overview to update as Runs and firings change, so that it is never
    stale while I watch it.

## Implementation decisions

**An Overview is computed, then drawn.** A new feature package, `internal/ui/overview`, holds
both halves. The computation is pure: it takes the Run records, the Automation entries and
their firing records, the Bot Profiles with their live sessions, the window and the clock, and
returns a value per Overview (Projects, their counts, buckets, funnel, durations, summaries;
Automations, their upcoming fires and recent outcomes; Profiles and their live Bots). It reads
no files, no store and no tmux. The drawing takes that value, a width, the glyph mode and the
theme, and returns content lines. The root's adapters feed it what they already load and hand
its lines to the content pane, the way `runContentLines`, `automationContentLines` and
`botsContentLines` do now for their header rows. The package name joins
`tools/architecture/check-file-names` and its imports are listed in
`check-ui-boundaries`; ntcharts is allowed for the drawing only.

**ntcharts joins the module.** ntcharts v0.5.x is built on the Bubble Tea v1 and lipgloss v1
line the manager uses. The Overview uses its static rendering (bar chart, sparkline, timeline)
as painters into strings, never as nested Bubble Tea models: an Overview takes no input of its
own besides the window and the scroll, which the root routes. The rune set is chosen from the
glyph mode: braille under Advanced, block and ASCII runes under Compatible.

**The Run record decodes four more fields.** `result.summary`, `prState`, `finishedAt` and
`agents` are already written by Fractory on every record that has them; the manager's `Record`
starts decoding them as optional. No `RecordVersion` change: an older record without them still
parses, and the Overview treats a missing field as absent (no duration without `finishedAt`, no
funnel stage without `prState`), never guessed.

**Project and Run Title come from Spec 0004's derivations.** The Project is the base name of a
Run's checkout path and the Run Title the record's title or its de-slugged branch, exactly as
draft Spec 0004 defines them; this Spec reuses those functions and defines none of its own. The
Project header's Mode, base branch and gates come from the rules the manager already reads from
each Project's `.fractory.json`.

**What counts.** A Run is in the window when it was last updated inside it. In flight is every
queued, running and needs-human Run, ignoring the window. Throughput and durations count Runs.
The funnel counts Tickets, each by its latest Run's `prState`: a Ticket with a pull request URL
is opened, `MERGED` is merged, `CLOSED` is closed unmerged. Throughput buckets are hours for
24h and days otherwise, merged into weeks when the days would not fit the block's width.

**Project colour.** A Project's colour is an index into the theme's swatches, chosen by a stable
hash of the Project name, so it survives restarts and theme changes without being stored.

**Automations by Project.** An Automation belongs to the Project named by the base name of its
configured repository path, the same rule a Run's Project follows; an Automation with no
repository path belongs to Elsewhere. Upcoming fires come from the same schedule calculation the rail's next-fire label
uses; recent outcomes from the same last-ten firings the listing already keeps.

**Window and scroll are view state on the Overview.** The window cycles 24h → 7d → 30d → all →
24h and starts at all; it lives in memory only, never in the store. The scroll offset resets
when the selection leaves the header. The window key is a new entry in the key table, read
from the store like every other binding, named in the footer and in `?`. The window label is a
content line with a hit target, so a click steps the window; the wheel over the content pane
scrolls the Overview. `TestEverySurfaceAnswersTheMouse` does not apply, since the Overview is
content, not a surface, but every key it adds has a click and the reverse.

**Refresh rides the existing polls.** Runs and Automation records are already read every two
seconds. The Overview recomputes when what it was given changes and when the window or clock
bucket moves, not on every frame; drawing at a width is cached per value.

**Slices.** One Spec, three Tickets in order: the Runs Overview (with the shared window,
scrolling, palette and ntcharts), then the Automations Overview (after the path fix), then the
Bots Overview.

## Testing decisions

A good test asserts what the owner reads: the text of the Overview at a width, with escapes
stripped. It never asserts a chart's exact runes, a column position or an intermediate struct,
because ntcharts owns the runes and the point is the reading.

Two seams, the highest available for each half.

**The computation, as table tests in `internal/ui/overview`.** Records in, Overview value out,
with a fixed clock. This pins the counting rules: a Run in and out of each window, in flight
ignoring the window, a Ticket with three Runs counted once in the funnel by its latest, a
missing `finishedAt` giving no duration, a missing `prState` giving no funnel stage, Project
order by latest activity, an Automation in a Project and one under Elsewhere, a disabled
Automation with no upcoming fires, a Profile with and without live Bots. Prior art: the record
and Ticket grouping tests in `internal/ui/runs`.

**The Overview, through the content pane.** Load records into a `Model`, select the section
header, render the content lines at a width, strip escapes and assert on text — as the Runs
adapter's tests already do with their `runDetail` helper. This carries the rest: the Project
header with Mode and gates, the summaries under their Titles, the window label and that the key
and a click on it both step the window, that the wheel and the keys scroll a tall Overview,
that a row under the header still shows its own detail, the empty states, and that Compatible
glyph mode draws no braille. Narrow and wide widths are the same test driven twice.

The record decoding of the four new fields is asserted where record parsing is tested today,
including an old record without them.

A change to the UI is verified against real sessions before it is called done: build the
binary, run it under a throwaway `HOME` on its own socket, with real Runs in the registry, and
read the frames back.

## Open questions

- How the stacked bars and the funnel look at a 30-column content pane. The tests pin that they
  read; the real-session check decides whether they look good, and the bucket-merging rule may
  move.
- Whether a Project whose `.fractory.json` cannot be read shows its block without the header
  details or is skipped. Proposed: shown, with the rules it could not read marked unknown, as
  the Run detail does today.
