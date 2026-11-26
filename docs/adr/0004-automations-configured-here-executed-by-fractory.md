# Automations are configured in the manager and executed by Fractory

An Automation is a schedule plus everything needed to run work once: where it runs, which
CLI runs it, its Mode, and a reference to the committed script or prompt that does the work.
The manager owns that configuration as a JSON document in `state.db`, holds the clock while
it is open, and visualizes what came back. It never executes an Automation. When an instant
comes due it drops the whole configuration as a versioned JSON request for Fractory, which
runs it through Sandcastle in a worktree and a container and writes a record the manager
reads.

The obvious design is the opposite one: the manager already spawns agents into tmux panes
and could put a clock on that. We chose sandboxing over attachability. The manager is Go
with no container runtime and no way to call Sandcastle, which is an npm package Fractory
imports in process, so a manager that executed Automations would have to grow a sandbox
stack of its own — a long way from a thin wrapper around the supported TUIs. Giving
execution to Fractory costs the thing a pane would have given us: an Automation cannot be
opened and talked to while it runs. A Debug Session covers the case it leaves behind, by
opening an ordinary chat under the project's group with the finished run's paths in its
prompt.

## Consequences

- Every Automation needs a git repository, because Sandcastle needs a worktree and the
  container image comes from that repository's `.fractory.json`.
- The JSON is a contract across two languages and two repositories. Fractory owns the
  schema in `src/contracts/`; the Go side mirrors it pinned to a version and refuses
  anything else, as `internal/ui/runs/registry.go` already does for a Run record.
- The request carries the resolved configuration, never an id. Fractory never reads
  `state.db`, so the manager stays the only writer of its own store.
- Firing latency is bounded by Fractory's tick, currently three minutes, and nothing fires
  at all while the manager is closed. A missed instant runs once when it next opens.
- Automation requests and records live in their own directories, not in the runs registry,
  so the Run contract and the Runs rail are untouched and the two can be pruned on
  different schedules.
