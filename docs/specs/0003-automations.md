# Spec 0003: Automations

Status: accepted (2026-11-26). Vocabulary: [GLOSSARY.md](../../GLOSSARY.md).

## Goal

The owner runs the same work on a rhythm — a weekly reconciliation, a nightly report, a
Monday-morning sweep of a repository — and today each one is a session they remember to
start. They want to describe that work once in Fractory Manager, say when it should happen,
and then see what it did without having been there.

An Automation is that description: a schedule plus where the work runs, which CLI runs it,
its Mode, and a reference to the committed script or prompt that does the work. The manager
remembers it, notices when it is due, and shows what came back. Fractory runs it in a
container, the way it runs everything else, so an Automation that goes wrong at 07:00 cannot
touch the owner's working tree. When one does go wrong, the owner opens a Debug Session: an
ordinary chat under that project's group, already pointed at the run that failed.

## Non-goals

- **Firing while the manager is closed.** The clock lives in the manager process. Nothing
  fires on a machine with no Fractory Manager running; a missed instant runs once when it
  next opens. No launchd agent, no systemd timer, no `crontab` entry.
- **Attaching to a running Automation.** Work runs in a container Fractory owns. There is no
  pane to enter and no conversation to join. A Debug Session after the fact is the whole of
  the interactive story.
- **A permissions model.** An Automation stores the flags its CLI launches with; whatever
  that CLI's own flag means is what it means. The manager does not decide what a scheduled
  agent may do.
- **The executing half.** Fractory's automation executor, its flow and its record writer are
  a sibling Spec in `frattezi/fractory`. This Spec delivers the configuration, the clock, the
  request contract and the whole view, tested against fixture files.
- **Turning an Automation into a Ticket or a Run.** An Automation Run is not a Run: no
  `ticketRef`, no pull request, and nothing in the runs registry.
- **Editing the work itself in the TUI.** The script and the prompt are committed files in a
  repository. The dialog chooses which one and when, never its contents.

## Solution

The owner presses a key in the rail to open the Automations dialog and fills in a name, a
directory, a kind (Agent or Script), the CLI to run it, its Mode, a cron expression, the
entry point, the launch flags and how many runs to keep. The Automation appears as a row in
its own rail section showing its name, its Mode, when it last ran and how that went, and when
it fires next. Selecting it fills the detail column with its recent runs.

While the manager is open it checks, on the same cadence as the Runs poll, whether any
enabled Automation is due. A due Automation whose previous instance is still queued or
running is skipped and recorded as such — never queued behind it. Otherwise the manager
writes the whole resolved configuration as a request file for Fractory and advances the
Automation's watermark, so the same instant cannot fire twice. Fractory picks the request up
on its next tick, runs it through Sandcastle in a worktree and a container, and writes a
record the manager reads on its next poll.

A run's output is a file on disk. `Enter` on a run opens it in a log tab, tailed while it
runs and readable long after it finished — the same tab the Runs view already uses. `D` on a
finished run opens a Debug Session: a new chat session, in the Automation's repository and
under that project's group, whose first prompt names the Automation, the instant it fired for,
its outcome and the paths to its log and its worktree. The agent reads what it needs; nothing
is pasted into the prompt.

Every key has a click and every click has a key, and both appear in the footer and under `?`.

## User stories

1. As the owner, I want to create an Automation from the TUI, so that a recurring chore is
   described once instead of remembered weekly.
2. As the owner, I want to give an Automation a name I choose, so that its rail row is
   recognisable at a glance.
3. As the owner, I want to say which directory an Automation runs in, so that its work and its
   container image come from the right repository.
4. As the owner, I want to choose whether an Automation is an Agent Automation or a Script
   Automation, so that a chore with a right answer runs as code and only an ambiguous one
   costs tokens.
5. As the owner, I want to choose which CLI runs an Agent Automation from the tools the
   manager already knows, so that a new CLI needs no change here.
6. As the owner, I want to set an Automation's Mode, so that one repository can hold both Work
   and Personal Automations.
7. As the owner, I want to write a cron expression and have it read in my own timezone, so that
   "07:00 daily" means what it says.
8. As the owner, I want an Automation's timezone recorded with it, so that travelling does not
   silently move when my chores run.
9. As the owner, I want to point an Automation at a committed script or prompt, so that the
   logic has a diff, a review and a history.
10. As the owner, I want to store the flags the CLI launches with, so that an Automation can
    run unattended without the manager inventing a policy for it.
11. As the owner, I want to enable and disable an Automation without deleting it, so that
    pausing a chore does not lose how it was set up.
12. As the owner, I want to edit an Automation, so that changing a schedule does not mean
    recreating it.
13. As the owner, I want to delete an Automation, so that a chore I no longer do stops firing.
14. As the owner, I want to manage Automations from a shell as well as the TUI, so that the
    CLI and the MCP server read and write the same values the dialog does.
15. As the owner, I want an Automation to fire when it comes due while the manager is open, so
    that I do not start it by hand.
16. As the owner, I want a missed instant to run once when the manager next opens, so that an
    overnight chore is not silently skipped and not run five times.
17. As the owner, I want a due instant skipped when the previous one is still working, so that
    a slow chore never doubles up on its own directory.
18. As the owner, I want a skipped instant recorded and visible, so that a gap in the history
    is explained rather than mysterious.
19. As the owner, I want the same instant never to fire twice even if the manager restarts
    mid-check, so that a chore with side effects is safe to schedule.
20. As the owner, I want to run an Automation now, outside its schedule, so that I can test it
    without waiting or editing its cron.
21. As the owner, I want Automations in their own rail section, so that they do not crowd the
    Tickets I am working on.
22. As the owner, I want each row to show the last outcome and the next fire time, so that one
    glance tells me whether my chores are healthy.
23. As the owner, I want to see an Automation's Mode on its row, so that Work and Personal
    work is never confused.
24. As the owner, I want the detail column to list an Automation's recent runs, so that I can
    see a pattern rather than only the latest result.
25. As the owner, I want to open a run's output in a log tab, so that I can read what it did
    while it is running and after it finished.
26. As the owner, I want a run's output to survive a manager restart, so that "what happened
    last Tuesday" is answerable.
27. As the owner, I want to open a Debug Session from a finished run, so that looking into a
    failure starts with the context already in hand.
28. As the owner, I want the Debug Session to open under that project's group, so that it sits
    with the rest of my work on that repository.
29. As the owner, I want the Debug Session's prompt to carry paths rather than pasted output,
    so that the agent can read everything and the prompt stays small.
30. As the owner, I want each Automation to keep only the last few runs, so that a daily chore
    does not accumulate a year of records.
31. As the owner, I want every Automation action reachable by key and by click, so that the
    feature works the way the rest of the manager does.
32. As the owner, I want the manager to refuse a malformed cron expression or a missing entry
    point when I save, so that I learn about it in the dialog and not three minutes later in a
    log.
33. As the owner, I want an Automation whose directory is not a Fractory repository to be
    refused with a reason, so that I do not schedule work that can never run.
34. As the owner, I want the Automations view to behave when Fractory has never run, so that an
    empty or absent registry shows an empty section rather than an error.

## Implementation decisions

**Store.** One new table, appended to the end of the migrations slice in `internal/store`,
holding one row per Automation: the stable id, a name the owner can change, the configuration as a JSON document, an
enabled flag, the `last_fired_for` instant, and created/updated stamps. The configuration is
a JSON document rather than a column per field because the same bytes travel into the request
file unchanged; the watermark and the enabled flag are columns because they are mutable state,
not configuration. Typed accessors live in `internal/store/automations.go` and encode time
through the existing `encodeTime`/`decodeTime`.

**Configuration contract, version 1.** `version`, a stable `id` that never changes, `name`, `kind` (`agent` | `script`),
`repoPath`, `cron`, `timezone` (an IANA name), `mode` (`work` | `personal`), `tool` (a name
from `builtinTools`), `flags`, `entry` (the committed script path, or the prompt reference for
an Agent Automation), `retention`.

**Request contract, version 1.** `version`, `requestedAt`, `firedFor`, and the configuration
verbatim under `automation`. Written atomically through `internal/atomicfile` into
`$FRACTORY_HOME/automations/`, one file per instant, named so that the same instant produces
the same filename and a repeat write is a no-op — the idempotency mechanism `.rerun.json`
already uses. Fractory never reads `state.db`; the manager stays the only writer of its own
store.

**Record contract, version 1.** Read-only, from `$FRACTORY_HOME/automation-runs/`:
`version`, `id`, `automationId`, `firedFor`, `status`, `startedAt`, `updatedAt`, `logPath`,
`result`, and the worktree path. Parsing refuses any other version and ignores unknown fields,
so Fractory can extend the contract — the rule `internal/ui/runs` already follows for a Run
record. Fractory owns the schema for all three contracts in its `src/contracts/`; the Go side
mirrors them pinned to a version.

**Clock.** Due-ness is a pure function of the enabled Automations, their watermarks, the set
of instances currently outstanding, and `now`. It returns the instants to fire and the ones to
record as skipped. It never reads a clock itself. Evaluated on the existing poll cadence; the
watermark is advanced in the same step that writes the request, so a crash between them
re-fires at most the one instant whose request file already exists, where the filename makes
the second write a no-op.

**Packages.** A new `internal/ui/automations` holds the contracts, the due computation and the
file IO, boundary-restricted to `internal/atomicfile` as `internal/ui/runs` is. A root adapter
`internal/ui/automations_adapter.go` owns the poll, the rail rows, the detail column, the log
tab and the Debug Session intent; the rail section lives in `internal/ui/rail`. Store, tmux and
Git work goes on the ordered effect lane. The dialog is a surface, not a Settings row: an
Automation is a record like a group or a connection, and the Settings modal takes values a user
steps through, not commands. `agent-manager automations` is the second front onto the same
command layer. `automations` is added to `tools/architecture/check-file-names`, and new
keybindings are named in the help catalog.

**Platform.** Nothing platform-specific is introduced. With the clock in-process there is no
OS scheduler to install, so the feature covers every platform the binary builds for, WSL2
included.

**Reuse, not new machinery.** The log tab is the Runs view's `tail -F` pane. The Debug Session
is an ordinary spawn through the existing lifecycle, with a group and a launch prompt. The
registry poll, the atomic file-drop and the version-pinned reader are all patterns the fork
already runs in production.

## Testing decisions

A good test here asserts external behaviour: the bytes written to a request file, which
instants a given clock state produces, and what the rendered frame says. None of it needs
Fractory, Docker or a network.

**The seam to aim for is `internal/ui/automations`**, exercised as a package with a temp
directory and an injected `now` — the shape `internal/ui/runs/rerun_test.go` already uses,
where a request is written, its exact bytes are compared against the contract, and a second
request proves the no-op. It is the highest seam that covers the feature's real correctness:
the schedule arithmetic, the skip-while-busy rule, the once-only watermark, the DST cases, and
the refusal of a record whose version is not 1. Fixture record files stand in for Fractory.

Two supporting seams, both with prior art: the store's typed accessors over a temp `state.db`,
and the root adapter driven as a `Model` with fixture Automations and records, as
`internal/ui/runs_adapter_test.go` drives the Runs rail — asserting the rail rows, the detail
column, that the log tab opens on the right path, and that the Debug Session spawn carries the
right group and prompt. The validation the dialog performs is tested through the adapter, not
the view.

Per this repo's rules the suite runs with `env -u TMUX TMUX_TMPDIR=/tmp/amtest` and
`SHELL=/bin/sh`, and the view is verified against real frames before the pull request claims
it works.

## Open questions

- Whether a skipped instant deserves its own record, or a note on the Automation's row. The
  first is more honest and more files; the second is cheaper and loses history.
- Whether retention is enforced by the manager or by Fractory. Fractory writes the records, so
  it is the natural pruner; the manager holds the retention number.
- Whether `run now` writes a request with the current instant as its `firedFor`, or a distinct
  manual marker so a forced run is never mistaken for a scheduled one in the history.
- What an Agent Automation's `entry` refers to exactly — a prompt file in the repository, or a
  skill name the agent invokes. Both satisfy "committed, with a diff".
