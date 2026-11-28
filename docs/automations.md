# Automations

Work that starts on a schedule without anyone asking. Vocabulary:
[GLOSSARY.md](../GLOSSARY.md). This page is the reference for the whole feature: the
dialog, the rail, the keys, the shell commands and the files on disk. Why execution lives
outside the manager is
[ADR 0004](adr/0004-automations-configured-here-executed-by-fractory.md); what the feature
set out to do is [Spec 0003](specs/0003-automations.md).

## What an Automation is, and what runs it

An Automation is a stored configuration: a cron expression, the repository the work runs
in, whether the work is an agent prompt or a committed script, which CLI runs it, its Mode,
the launch flags and how many firings to keep. The manager owns that configuration in
`state.db`, holds the clock while it is open, and shows what came back. **It never executes
an Automation.** When an instant comes due it writes the whole configuration as a versioned
JSON request under `$FRACTORY_HOME/automations/requests/`, and Fractory picks it up on its next tick
and runs it through Sandcastle in a worktree and a container. Fractory writes a record under
`$FRACTORY_HOME/automations/<automationId>/`, which the manager reads on its next poll.

Three consequences are worth knowing before you create one:

- **Every Automation needs a Fractory repository.** The directory is refused unless it holds
  a readable `.fractory.json` object: that is where Fractory reads the image and the rules a
  firing needs.
- **Nothing fires while the manager is closed.** The clock is the manager process. There is
  no launchd agent, no systemd timer and no `crontab` entry. An instant missed overnight
  fires **once** when the manager next opens, not once per instant missed.
- **A firing cannot be attached to.** It runs in a container Fractory owns, so there is no
  pane to enter. Reading its log while it runs, and a Debug Session after it finishes, is
  the whole of the interactive story.

Fractory's home is `$FRACTORY_HOME`, else `~/.fractory`. With neither resolving, the manager
has nowhere to write a request and says so instead of firing.

## Creating one

`S` from any row in the list opens the Automations dialog; **New Automation** is in the row
menu of the Automations header too. The dialog's fields, in focus order:

| Field | What it takes |
|---|---|
| name | What the row shows. Yours to choose and to change later |
| directory | Absolute path of the Fractory repository the work runs in |
| kind | `agent` (the work is a prompt given to a CLI) or `script` (the work is a committed command) |
| CLI | The CLI that runs an Agent Automation, from the tools the manager knows. A Script Automation may leave it empty |
| mode | `work` or `personal`, so one repository can hold both |
| cron | Five fields, read in the timezone below (see [Cron](#cron)) |
| timezone | An IANA name such as `Europe/Lisbon`. `Local` is refused, so travelling never silently moves when a chore runs |
| entry | The committed file the work is: a prompt for an Agent Automation, a script for a Script one |
| flags | The launch flags the CLI starts with, space separated. Whatever that CLI's flag means is what it means; the manager has no policy of its own |
| keep runs | How many firings to keep, at least 1. New Automations start at 10 |

`tab` and `↑↓` move between fields, `←→` change a stepped one (kind, CLI, mode), `enter`
saves and `esc` cancels. A save is refused against the field that is wrong — a malformed
cron expression, a timezone that is not an IANA name, a directory that is not a Fractory
repository — so you learn about it in the dialog rather than three minutes later in a log.

`r` on an Automation row reopens the dialog on what is stored. Its id never changes.

## The rail

Automations have their own section in the list, below the sessions, so they never crowd what
you are working on. Each one is two lines: the fold mark, a status mark painted from the last
firing, the name and the Mode badge, then the last outcome and when it fires next
(`next 07:00` today, `next Mon 07:00` within the week, `next Jan 2` after that). A disabled
Automation is muted and says `disabled` in place of its next instant; one that never fired
says `never fired`; one whose stored configuration cannot be read says `unreadable` and
still shows, so it can be edited or deleted.

Unfolding an Automation lists its **recent firings**, newest first, up to ten: the instant it
fired for and how it went. Selecting an Automation fills the detail column with its state,
Mode, kind, schedule and timezone, next instant, directory, CLI, entry point and that list of
firings with their durations. Selecting one firing shows its status, outcome, summary,
start, duration and the paths to its log and its output directory.

`/` searches Automations by name, Mode and last outcome along with everything else, and the
attention filter (`w`) keeps the ones whose last firing ended `failed` or `needs-human`.
The archived view has no Automations section.

## Keys and clicks

Every key here is in the row menu (`[…]` or right click) as well, and under `?`.

| Key | On | What it does |
|---|---|---|
| `S` | any row | New Automation |
| `enter` | the header, or an Automation | Fold or unfold it |
| `enter` / `A` | a firing | Open its log in a tab |
| `A` | an Automation | Open its newest firing's log |
| `r` | an Automation | Edit it; the dialog opens on what is stored |
| `E` | an Automation | Enable or disable it. A disabled Automation keeps its setup |
| `!` | an Automation | Run it now for the current instant; its schedule is untouched |
| `d` | an Automation | Delete it, after a confirmation |
| `D` | a finished firing | Open a [Debug Session](#debug-sessions) on it |

A log tab is the same `tail -F` pane the Runs view uses: it follows a running firing and
stays readable long after it ended. A firing whose record names no log says so instead of
opening an empty tab. Clicking a fold mark folds, and a double click on a firing opens its
log.

## When an Automation fires

The check runs on the Runs poll cadence (every 2s) while the manager is open, and once at
startup, which is what fires an instant that passed while it was closed. Each enabled
Automation carries a **watermark**: the instant it last fired for, or the instant it was
created, so adding one never sets it off immediately.

- An Automation whose next instant after its watermark has passed fires **once**, for the
  most recent instant it missed.
- A due Automation whose previous firing is still `queued` or `running`, or whose request
  Fractory has yet to take, is **skipped**: it is never queued behind itself. The watermark
  advances anyway, so a slow chore does not pile up on its own directory.
- The request is written first and the watermark advanced second. A crash between the two
  leaves the request on disk, and the file is named after the Automation's id and the instant,
  so writing it again is a no-op. **The same instant never fires twice.**
- A request that cannot be written leaves the watermark where it was, and the next check
  tries again. What a check could not do is reported once on the status bar, not on every
  check.
- An Automation disabled, edited or deleted between a check reading the store and the firing
  writing its request is left alone: the row is read again first.

Firing latency is bounded by Fractory's tick, currently three minutes. `!` (**Run now**)
writes a request for the current instant without touching the watermark, so the next
scheduled instant still fires; it is skipped on the same busy rule, and the status bar says
so.

## Cron

Five fields: minute, hour, day of month, month, day of week. Each takes `*`, values, ranges
(`1-5`), lists (`1,15`) and steps (`*/15`, `1-5/2`). Month and weekday names are accepted
(`jan`, `mon`), case-insensitively, and `7` is Sunday as well as `0`. When both day fields
are restricted the schedule matches **either**, as every cron does; when one is `*` it
matches both. There are no `@daily`-style shorthands and no seconds field.

The expression is read in the Automation's timezone, which is why the timezone is stored
beside it rather than taken from the machine. Across a DST change: a wall time the clocks
skip over fires at the instant the gap ends, and one they repeat fires on its first
occurrence only.

## Debug Sessions

`D` on a finished firing opens an ordinary chat session in the Automation's repository, under
that project's group — the live group whose default directory is that repository, the
shallowest when several are, and otherwise a new group named after the repository. It is
named after the Automation and the instant (`debug-weekly-report-20261130-0700`) and runs on
the Automation's CLI, or the picker's default for a Script Automation.

Its first prompt names the Automation, its kind, its entry point, the instant it fired for,
the outcome, and the paths to the log and the output directory. **It carries paths, never
pasted output**, so the agent reads what it needs and the prompt stays small. The worktree is
gone by then; the output directory, which Fractory mounts into the container, stays on the
host.

A firing that has not finished has no Debug Session: there is nothing to look into yet, and
the refusal points at its log instead.

## From a shell

The same command layer the dialog uses, so the CLI reads and writes exactly what the dialog
does.

```
agent-manager automations list [--json]
agent-manager automations show <id|name>
agent-manager automations add [--disabled] [file|-] [--json]
agent-manager automations change <id|name> [file|-] [--json]
agent-manager automations remove <id|name> [--json]
agent-manager automations enable <id|name> [--json]
agent-manager automations disable <id|name> [--json]
```

An Automation is addressed by id, or by a name exactly one Automation carries; an ambiguous
name is refused with the ids to choose from. `add` and `change` read a configuration document
from the file named or from stdin. `change` keeps the id, the enabled flag and the watermark;
`show` prints the configuration with that state beside it.

The configuration contract, version 1. Every field is written into the request unchanged, so
the names are Fractory's, and **a document carrying any other name is refused**:

```json
{
  "version": 1,
  "name": "weekly report",
  "kind": "agent",
  "repoPath": "/Users/me/work/acme",
  "cron": "0 7 * * 1",
  "timezone": "Europe/Lisbon",
  "mode": "work",
  "cli": "claude",
  "flags": [],
  "entryPoint": "prompts/weekly.md",
  "keepRuns": 10
}
```

`version` may be left out, in which case it is 1; a document that names another version is
refused rather than guessed at. `id` may be left out of an `add`, and the manager assigns one;
on a `change` it is either absent or the Automation's own. Everything else is required.

```bash
agent-manager automations add weekly.json          # or: … add - <<<"$json"
agent-manager automations disable "weekly report"
```

## The files on disk

Three versioned JSON contracts, all under Fractory's home. Fractory owns the schemas in its
`src/contracts/`; the Go side mirrors them pinned to version 1 and refuses anything else.
Fractory never reads `state.db`, so the manager stays the only writer of its own store.

| Directory | Written by | Holds |
|---|---|---|
| `$FRACTORY_HOME/automations/requests/` | the manager | One request per instant, `<id>@<YYYYMMDDTHHMMSSZ>.json`: `version`, `requestedAt`, `firedFor` and the configuration verbatim under `automation`. Written atomically, so Fractory never reads half a file |
| `$FRACTORY_HOME/automations/<automationId>/` | Fractory | One record per firing: `version`, `id`, `automationId`, `firedFor`, `status`, `startedAt`, `updatedAt`, `logPath`, `outputPath` and `result` (`outcome`, `summary`) |

A record whose version is not 1, or which is missing `id`, `automationId`, `firedFor` or
`status`, is skipped rather than shown wrong; unknown fields are ignored, so Fractory can
extend the contract without a manager release. A missing or empty directory is an empty
section, not an error — the view behaves on a machine where Fractory has never run.

## Trying it without Fractory

The records are files, so the whole view can be exercised with a fixture. Point the store and
Fractory's home at a throwaway directory and nothing of yours is touched:

```bash
make build
dir=/tmp/am-automations && mkdir -p "$dir/cfg" "$dir/sock" "$dir/home/automations/requests"
env -u TMUX -u AGENT_MANAGER_SESSION_ID \
    AGENT_MANAGER_CONFIG_DIR="$dir/cfg" TMUX_TMPDIR="$dir/sock" FRACTORY_HOME="$dir/home" \
    ./agent-manager
```

Run that in a terminal of its own: the manager needs a real TTY. From a second shell, the
same variables reach the same store, which is how you add an Automation and drop a record for
it while the manager is open. Write a record with the `automationId` that
`agent-manager automations list` printed, and the row picks it up on the next poll, with its
log tab reading whatever `logPath` names. Delete `$dir` when you are done.

Point `repoPath` at a real repository with a `.fractory.json`, or `add` is refused.

## What is not here yet

- **The manager does not prune.** `keepRuns` travels into the request; Fractory writes the
  records, so it is the one that removes them.
- **A skipped instant leaves no record**, only an advanced watermark, so a gap in the history
  is visible as a missing firing rather than explained in place.
- **A forced run is not marked.** `!` writes the current instant, so a manual firing reads
  like a scheduled one in the history.
- **No MCP tools.** Automations are managed from the TUI and the `agent-manager automations`
  subcommands; agents do not reach them the way they reach
  [each other](agent-to-agent.md).
