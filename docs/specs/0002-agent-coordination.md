# Spec 0002: Agent coordination — threaded asks, dispatch and report-back

Status: draft.

## Goal

A session can ask another session a question and get the answer back, and a session that
spawns workers gets their results without reading their panes. Today the manager has a
careful one-way postal service: `send_session` writes a row to `session_inbox`, and the
poller types it into the recipient only when that pane is genuinely at rest. What it has
no notion of is a *conversation*. There is no reply-to and no thread; `answered` is
inferred from "the recipient later sent me anything at all"; there is no way to wait for
an answer, because `wait_for_session` waits on a status column; and a spawned session owes
its parent nothing.

The consequence is that coordination between sessions is carried by convention instead of
by the product. A session that fans work out has to poll `read_session` and parse another
agent's prose to decide whether the work is done, and a session that needs an answer has
to send a message and then guess when to look. Both are steps that a user currently has
to remember to ask for, and that no code enforces. After this Spec, a question carries an
identity, an answer finds its way back to whoever asked, and a dispatched session reports
a result that the manager assembles from what it already knows.

## Non-goals

- **Cross-host coordination.** `ask`, dispatch and report-back are local-host only, behind
  the existing `localOnly` guard. A thread's rows live in the database that minted them,
  and making a thread span two SQLite files is a separate project. Cross-host messaging
  stays what it is today: a plain message with no thread.
- **A human decision queue.** Surfacing "this session is blocked on a person, and here is
  its question" is a view, and it belongs with the other view work, not in this protocol.
- **Terminals as participants.** A session whose tool is `Shell` can be sent to, never
  asked or dispatched. This is already the behaviour of `runtime.agent`.
- **Changing when delivery happens.** The rules that decide whether a pane may be typed
  into are unchanged. This Spec changes what a message carries and the order in which
  queued messages go, not the gates.
- **A permission model.** Any session may still message, archive or kill any other.
  Parentage becomes visible and drives defaults; it does not become a boundary.
- **A longer wait.** The five-minute wait cap stays. A timeout is a normal outcome.
- **Replacing the task board.** `tasks` and `task_deps` keep their current role: who does
  which piece of work. A thread is a conversation about work, not the work item.
- **An inbox view.** Showing who messaged whom and what still waits is already in flight
  on `feat/message-watch-view`, which also introduces the message `subject` this Spec
  reuses as a thread's subject.

## Solution

**A thread.** Every message may belong to a thread, named by an opaque id. A session that
sends a message and wants a reply marks it so, and gets the thread id back. A session
that answers quotes the thread id. `message_status` then reports `answered` because an
answer exists on the thread, not because the recipient happened to send something back.
A message with no thread behaves exactly as it does today.

**An answer returns through the tool call.** This is the load-bearing decision. A session
waiting for an answer is sitting inside a tool call, which reads as `working`, and the
poller only delivers to a pane that is at rest — so an answer delivered the normal way
would wait for the asker to stop waiting, while the asker waits for it. Instead, the
waiting call reads the answer out of the store and returns it as its own result. Nothing
is typed into the asker's pane. If the wait has already timed out when the answer arrives,
the answer reverts to an ordinary queued message and is delivered the usual way, so an
answer is never lost because a question stopped waiting. An ask sent without waiting is
therefore safe: it degrades into an ordinary message that happens to carry a thread.

**A wait that cannot deadlock.** A session waits on at most one thread at a time, because
it has at most one tool call open. That makes "who waits on whom" a graph with one edge
per session, and a cycle is found by following at most as many edges as there are
sessions. A wait that would close a cycle is refused at call time, with an error naming
the other session, rather than leaving two sessions to time out.

**A dispatch that owes a result.** `create_session` may declare that the new session owes
its caller a result. The manager mints a thread for the dispatch, records it on the child,
and tells the child in its launch prompt that it was dispatched and how to finish. The
child finishes by calling `worker_done` with one short summary. The manager attaches the
facts it already holds — the worker's branch and worktree, its final status, and the
outcome of any task it claimed — and enqueues the whole thing as the answer on the
dispatch thread. The coordinator waits on that thread and gets a result, instead of
waiting on a status and reading a pane.

**A dispatch that dies still answers.** If a session with an open dispatch thread goes
`dead` before calling `worker_done`, the poller closes the thread itself: an answer whose
outcome is `died`, carrying the last status and the stored pane snapshot. No model writes
that failure.

**Defaults from parentage.** `spawned_by` already exists in the database and is surfaced
nowhere. `list_sessions` starts reporting it, an `ask` with no target goes to the caller's
spawner, and `worker_done` reports to whoever dispatched the caller.

**Limits that fit replies.** The existing caps — twenty queued per recipient, five
messages a minute per sender pair, a ten-minute duplicate window on the body — are wrong
for replies. A worker reporting back on six fanned-out tasks would trip the rate cap, and
two workers reporting the same short summary on different threads would be swallowed as
duplicates. So a message that quotes a thread skips the rate cap, and the duplicate
fingerprint includes the thread id. An ask that starts a thread keeps the full existing
budget, because that is the direction unsolicited volume travels.

**Priority.** The queue is ordered by kind before id: answers first, then asks, then plain
messages. Someone is blocked behind an ask and nobody is blocked behind a tell.

**Observability.** The protocol emits lifecycle events — `dispatch`, `ask`, `answer`,
`worker_done` — alongside the existing `task_claim` and `reservation_set`. Nothing in the
manager consumes them yet; they exist so that coordination is visible from outside without
the manager growing a notification feature, and so that a later view has a feed to draw.

**How a session learns the protocol.** The guide is printed by the binary that implements
it, and a stub skill resolves it. The repository already ships `skills/agent-manager` and
`skills/agent-manager-reference`, but both are static documents served from the website and
sealed by digest, which is the wrong shape for a protocol whose details move with the
binary. `agent-manager coordination guide` prints the guide for the running build; the stub
says little except how to ask for it.

## User stories

1. As a session that needs a fact only another session has, I want to send it a question
   and be told the thread it opened, so that I can wait for exactly that answer.
2. As a session that asked a question, I want to wait for the answer and receive it as the
   result of my own call, so that I am not waiting for a message that cannot reach me.
3. As a session that asked a question, I want my wait to end with a plain `timed_out`
   rather than an error, so that I can do other work and wait again.
4. As a session whose wait timed out, I want the answer to arrive in my inbox when it is
   finally sent, so that waiting is an optimisation and never the only path.
5. As a session that was asked a question, I want to answer by quoting the thread, so that
   my reply reaches the asker's waiting call and not its prompt.
6. As a session that sent a question without waiting, I want it to behave like any other
   message, so that threading never obliges me to block.
7. As a session about to wait on another that is already waiting on me, I want the call
   refused immediately with the other session named, so that I learn about the cycle
   instead of timing out.
8. As a session that sent a message, I want `message_status` to tell me `answered` only
   when an answer exists on the thread, so that the status means what it says.
9. As a session that sent a message with no thread, I want everything to behave exactly as
   it does today, so that threading is additive.
10. As a session coordinating work, I want to spawn a session that owes me a result, so
    that I do not have to poll its pane to find out whether it finished.
11. As a dispatched session, I want my launch prompt to tell me that I was dispatched and
    how to report, so that the obligation is in my context rather than in a convention.
12. As a dispatched session, I want to finish by writing one short summary, so that the
    facts about my branch, worktree and tasks are not mine to restate.
13. As a coordinating session, I want a worker's result to carry its branch, worktree,
    final status and task outcomes, so that I can act on facts rather than on prose.
14. As a coordinating session, I want a worker that dies to still close its thread with a
    `died` outcome and its last pane snapshot, so that I am never waiting on a session
    that no longer exists.
15. As a coordinating session, I want to wait on the dispatch thread rather than on the
    worker's status, so that I am woken by a result and not by an idle pane.
16. As a dispatched session, I want an `ask` with no target to reach whoever dispatched me,
    so that asking upward needs no bookkeeping on my part.
17. As any session, I want `list_sessions` to show which session spawned each row, so that
    I can tell a coordinator from a worker.
18. As a worker reporting on several dispatches at once, I want my replies exempt from the
    per-minute rate cap, so that reporting back cannot be throttled into failure.
19. As one of several workers reporting the same short summary, I want my reply judged
    distinct because its thread is distinct, so that it is not discarded as a duplicate.
20. As a session with an answer and a plain message both queued, I want the answer first,
    so that whoever is blocked moves before whoever is not.
21. As a session whose peer is archived, dead or sitting on a dialog, I want an ask to
    report `held` with the reason, exactly as a message does today.
22. As an operator, I want `dispatch`, `ask`, `answer` and `worker_done` to fire lifecycle
    events, so that my hooks can act on coordination without the manager notifying anyone.
23. As an operator, I want every new capability reachable from `agent-manager …` as well as
    from the MCP tools, so that a CLI without an MCP client is not a second-class
    participant.
24. As a session running any supported CLI, I want to read the coordination guide from the
    binary I am running under, so that the guide cannot drift from the protocol.
25. As an operator, I want `ask`, dispatch and `worker_done` refused across hosts with a
    clear message, so that I am not left with a thread that can never be answered.
26. As an operator, I want a terminal to remain unaskable and undispatchable, so that the
    protocol stays between sessions that can answer.

## Implementation decisions

### Schema

Append-only entries on the `migrations` slice in `internal/store`, each harmless to re-run:

```
ALTER TABLE session_inbox ADD COLUMN thread_id    TEXT    NOT NULL DEFAULT ''
ALTER TABLE session_inbox ADD COLUMN in_reply_to  INTEGER NOT NULL DEFAULT 0
ALTER TABLE session_inbox ADD COLUMN kind         TEXT    NOT NULL DEFAULT 'tell'
CREATE INDEX session_inbox_thread ON session_inbox (thread_id, id)
CREATE INDEX session_inbox_order  ON session_inbox (session_id, delivered_at, kind, id)
ALTER TABLE sessions ADD COLUMN dispatch_thread_id TEXT NOT NULL DEFAULT ''
ALTER TABLE sessions ADD COLUMN report_to          TEXT NOT NULL DEFAULT ''
CREATE TABLE IF NOT EXISTS inbox_waits (
  session_id TEXT PRIMARY KEY,
  thread_id  TEXT    NOT NULL,
  target_id  TEXT    NOT NULL,
  since      INTEGER NOT NULL
)
```

`kind` is `tell`, `ask` or `answer`. `thread_id` is empty for an unthreaded message, which
is every row that exists before the migration. `in_reply_to` holds the `session_inbox.id`
the reply answers, which is what makes an answer attributable to a specific question rather
than to a thread that may carry several.

`inbox_waits` has one row per waiting session, because a session has at most one open tool
call. That is what makes the waiting graph a functional graph and cycle detection a walk
rather than a search. The row is written when a wait begins and deleted when it ends, by
the same call, so a crashed session leaves a stale row that is pruned on next read by
checking the session is still alive.

### Store

- `Enqueue` gains the three fields. The rate cap is skipped when `in_reply_to != 0`. The
  dedupe `fingerprint` becomes the hash of the whitespace-collapsed body *and* the thread
  id, so identical bodies on distinct threads are distinct.
- `ClaimMessage`'s ordering changes to `ORDER BY CASE kind WHEN 'answer' THEN 0 WHEN 'ask'
  THEN 1 ELSE 2 END, id`, served by `session_inbox_order`.
- `ClaimAnswer(threadID, waiterID, at)` — the second claimant on an answer row. The same
  compare-and-set as `ClaimMessage`, so an answer is taken either by the waiting call or by
  the poller, never both. It marks the row delivered without a pane submission.
- `Answered(threadID)` reports whether an answer exists, which is what `message_status`
  now reads instead of `read_at`. `read_at` keeps its current meaning for unthreaded
  messages, so nothing existing changes.
- `BeginWait`, `EndWait`, `WaitCycle(sessionID, targetID)` on `inbox_waits`. `WaitCycle`
  follows the single outgoing edge from `targetID` until it finds `sessionID` (a cycle),
  finds a session with no wait, or exceeds the session count.

### `internal/sessioncmd`

All behaviour and validation lives here, as it does today, so the CLI front in
`internal/cli` gets it for free and `parity_test.go` keeps the two paired.

- `Sessions.Send` gains `ExpectReply bool` and `InReplyTo string`. With `ExpectReply` it
  mints a thread and returns it on `SendResult`. With `InReplyTo` it sets `kind = answer`
  and resolves the thread, refusing a thread the caller is not a party to.
- `Sessions.WaitForAnswer(threadID, timeout)` — refuses a thread the caller did not open,
  refuses a cycle via `WaitCycle`, writes `inbox_waits`, polls `ClaimAnswer` on the
  existing `config.PollInterval`, and returns `AnswerResult{Answer, Outcome}` with
  `Outcome ∈ {answered, timed_out, died}`. The five-minute `MaxWaitTimeout` is reused.
- `Sessions.Create` gains `ReportResult bool`. It mints the dispatch thread, writes
  `dispatch_thread_id` and `report_to` on the child, and appends the obligation to the
  child's `launch_prompt`.
- `Sessions.WorkerDone(summary)` — requires the caller to have an open
  `dispatch_thread_id`, assembles the result, enqueues it as the answer, and clears the
  column so a second call is refused.
- `Sessions.List` reports `spawned_by`. An `ask` with an empty target resolves to the
  caller's `spawned_by`, and fails with a clear message when the caller has none.
- `localOnly` covers the new calls, joining `wait_for_session`, `task` and
  `review_comment`.

The worker result is a struct, not prose: `summary` from the worker, and `branch`,
`worktree`, `status`, `snapshot`, `tasks` and `outcome` assembled by the manager from
`worktree_repo`, `worktree_branch`, the status column, the stored `snapshot` and the
caller's settled tasks. `outcome` is `done` or `died`.

### Poller

`internal/execution` closes orphaned threads: on the transition that writes `dead`, a
session with a non-empty `dispatch_thread_id` has its thread closed with an `outcome:
died` answer carrying its last status and `snapshot`, and the column cleared. The delivery
gates in `polldelivery.go` are untouched.

### MCP and CLI surface

Two new tools, three extended, every one registered through `addTool` so the no-caller
refusal keeps covering them:

- `send_session` — `+ expect_reply`, `+ in_reply_to`; returns `thread_id`.
- `create_session` — `+ report_result`.
- `list_sessions` — row gains `spawned_by`.
- `wait_for_answer` — new.
- `worker_done` — new.

`message_status` reports the thread and its answer. Each gets its `agent-manager …`
counterpart.

### Lifecycle events

`dispatch`, `ask`, `answer`, `worker_done` added to `internal/lifecyclehook`, emitted from
`internal/sessioncmd` the way `emitTask` and `emitReservation` already are.

### The guide

`agent-manager coordination guide` prints the guide compiled into the binary.
`skills/agent-coordination/SKILL.md` is a stub whose body is how to resolve and run that
command. Nothing describing the protocol is duplicated in a static document.

### Coverage

The protocol is tool-agnostic: it is MCP tools and CLI subcommands, and the dispatch
obligation rides on `launch_prompt`, which every tool already receives. What is *not*
tool-agnostic is whether a given CLI reads the guide and honours the obligation, so the
matrix work is verifying the dispatch prompt and the guide against each CLI in
`builtinTools` on real sessions, not a green suite.

## Testing decisions

A good test here asserts external behaviour: what a caller gets back, what lands in a
recipient's queue, and in what order — never how a thread is stored.

**One behavioural seam: `internal/sessioncmd`.** It already holds every rule, its tests
already drive a real SQLite store, and `internal/cli/parity_test.go` already forces the
MCP and CLI fronts to stay paired, so one seam covers both surfaces. The cases that matter
are the ones the design turns on: an answer reaching a waiting call rather than a pane; an
answer arriving after a timeout being delivered normally instead; a refused cycle naming
the other session; a dispatched worker's result carrying manager-assembled facts; a dead
worker's thread closing itself; a reply skipping the rate cap; two identical summaries on
distinct threads both surviving; an unthreaded message behaving exactly as before.

**Two algorithms tested directly in `internal/store`**, because they are computation rather
than behaviour, the way the reservation overlap logic is tested today: `WaitCycle` on the
waiting graph, and the queue ordering by kind.

**Real sessions**, as `AGENTS.md` requires for a change to the poller: a coordinator
dispatching a worker under a throwaway `HOME` on its own socket, the worker reporting back,
and a second worker killed mid-run to prove the `died` answer.

## Open questions

1. Should a thread carry more than one question and answer, or close after the first
   answer? The schema allows a conversation; nothing in this Spec needs one.
2. Should `worker_done` be refused when the worker's tasks are unsettled, or report them as
   unfinished and let the coordinator decide? Reporting is the lighter choice.
3. Does the dispatch obligation belong in `launch_prompt` or in a tool the worker is told
   to call first? The prompt is simpler and costs no round trip.
4. Should a stale `inbox_waits` row be pruned by the poller instead of on read, so the
   graph is clean without a reader paying for it?
