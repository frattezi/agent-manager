# Agents reaching each other

How one session in Fractory Manager sends work, questions and results to another.
Vocabulary: [GLOSSARY.md](../GLOSSARY.md).

## A note on names

The product is **Fractory Manager** — that is the wordmark in the header. The binary and
every command are still `agent-manager`, and so is the config directory. Renaming them is
not done, so every command below is written as it actually runs.

Each capability has two fronts onto the same code: an **MCP tool**, which is what a CLI
with an MCP client calls, and an **`agent-manager …` subcommand** for one without. They
are kept paired by `internal/cli/parity_test.go`, so neither front is behind the other.

## Who may call what

Almost every command acts as **the session whose pane ran it**. Run them from inside an
agent's own shell, not from a daemon or a script that outlived its pane. Commands that
need that caller refuse to run without one — `task` and `reserve` among them.

A few commands also work from a plain shell with no session: `sessions`, `read`, `wait`,
`spawn`, `kill`, `delete`, `revive`, `archive`, `move`, the group commands, and `send`
with `--from <name>`. A message sent that way says in its envelope that no reply can
reach the sender.

A terminal (a session whose tool is a shell) can be sent to, but never messaged as an
agent: the agent commands refuse it.

## Finding each other

```
agent-manager sessions [--json]
```

Every agent session with its id, CLI, group, directory and status. A session is addressed
by **id**, never by name. Your own id arrives in `AGENT_MANAGER_SESSION_ID`, set when the
manager spawns you.

```
agent-manager read <id> [--styled]
```

What that session's screen shows right now. For a dead session it returns the stored
snapshot of its last screen.

## Messaging

```
agent-manager send <id> "<message>"
agent-manager send <id> "<message>" --from <name>     # from outside a session
agent-manager message-status <message-id>
```

A message is **queued, never typed immediately**. Fractory Manager's poller types it into
the recipient once that pane is at rest, which is what stops a message landing on an
approval dialog or overwriting a half-typed line. It is delivered **at most once**: a
delivery that fails is marked dropped and never retried.

`message-status` reports one of:

| State | Meaning |
|---|---|
| `queued` | waiting for the recipient to be at rest |
| `held` | the recipient cannot be typed into at all, with the reason |
| `delivered` | typed into the recipient's prompt |
| `dropped` | could not be typed; it will never retry, so send it again |
| `answered` | the recipient has since sent you a message |

`answered` is inferred, not tracked: it means the recipient later sent you *anything*, not
that they answered this. Treat it as "they have since spoken", not as a reply receipt.

Limits, all per recipient and enforced on insert:

- **8,000 bytes** a message. Point at a file or a task for the detail.
- **20 undelivered** messages for one recipient, counted across every sender. A recipient
  that stays unreachable fills this and then refuses everyone.
- **5 messages a minute** from one sender to one recipient.
- **A duplicate body** from the same sender to the same recipient inside **10 minutes** is
  refused. Reword it, or say something new.

Every delivered message arrives inside a fence naming its sender, and says that commit,
push, merge, publish and delete still wait for the person.

## Waiting instead of polling

```
agent-manager wait <id> [--until <state>] [--timeout <duration>]
```

Parks until that session stops working, rather than reading its screen in a loop. The
default waits for any resting state: `finished`, `waiting`, `idle`, `errored`, `dead`.
Timeout defaults to 50s and caps at 5m; exceeding it exits non-zero, which is a normal
answer, not a failure.

A wait counts from **your own message**: if you sent something still queued, the target
being idle does not end the wait, so you cannot mistake its pre-message rest for a reply.

## The shared work list

Caller-only. Any session may create a task and any may claim one; there is no assignment.

```
agent-manager task list
agent-manager task create "<title>" [--body <text>] [--depends-on <id,id>]
agent-manager task claim [<task-id>]      # omit the id to take the oldest unblocked one
agent-manager task finish <task-id>
agent-manager task release <task-id>
agent-manager task delete <task-id>
```

Dependencies are enforced: a task whose dependencies are not all `done` cannot be claimed,
and the claim is refused in SQL, so two sessions racing cannot both take it. Only the
session holding a claim may finish or release it.

## File reservations

Caller-only. Advisory leases, so sessions sharing a checkout can see who is editing what.
Never enforced against a writer.

```
agent-manager reserve <path>... [--mode exclusive|shared] [--note <text>] [--ttl <duration>]
agent-manager reservations
agent-manager release-files [<path>...]
```

Default TTL is 30 minutes, capped at 4 hours.

## Starting another session

```
agent-manager spawn [--name <name>] [--prompt <text>] [--tool <cli>] [--worktree] [--json]
```

The new session cannot see your conversation, so the prompt must state the whole task.
Repo work takes `--worktree` so parallel sessions never share a checkout. A spawned
session owes you nothing: there is no result reported back, and nothing marks it as
working for you. Closing that gap is [Spec 0002](specs/0002-agent-coordination.md).

## What is not here yet

- No thread on a message, so no reply is attributable to a question.
- No way to wait for an **answer**; waits are on status only.
- No obligation on a spawned session to report a result.
- No cross-host threading: `send` crosses hosts, but nothing that waits does.

All four are the subject of [Spec 0002](specs/0002-agent-coordination.md).
