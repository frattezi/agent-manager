# Agent-authored surfaces are described in A2UI and rendered here

An agent that needs the owner to see or decide something richer than prose describes an
Agent Surface in [A2UI](https://a2ui.org) and writes it to a file. The manager parses that
file on its poll and draws it in a surface of its own. Nothing is painted inside a
session's pane, which belongs to that session's CLI, and no process other than the manager
renders anything, because the manager is the only one that owns a screen.

Three alternatives were weighed. **A widget per feature**, which is what the manager does
today, means every new kind of ask is a Go change and a release; an agent cannot invent a
question shape the manager has not already been taught. **The agent emitting terminal
escapes** was rejected outright: a session's pane is the CLI's, and a Run executes in a
container with no terminal at all. **Adopting `a2tea`**, the A2UI renderer for Bubble Tea,
is the obvious third option and is where this will end up — but it requires Bubble Tea v2,
and this fork runs v1 across 288 files, with `tea.MouseMsg` split four ways in v2 across
the 49 files that carry this product's "every key has a click" promise. Paying that
restructure to render a grilling form, before either planning view ships, puts the risk in
the wrong place.

So the contract is the **format**, not the library. The manager renders the subset its own
surfaces use with the lipgloss and bubbles already in `go.mod`. `a2tea` is adopted when
upstream moves to Bubble Tea v2 and the merge is free, or when a feature is visibly held
back by the subset, or when the fork breaks from upstream for a reason of its own. Because
every surface on disk is already A2UI, that swap deletes a renderer and changes no surface.

Deterministic views stay plain Go. The Runs rail, the Tickets view and Automations are
computed from files on disk: a view a model repaints costs tokens for what code decides,
cannot be asserted on its output, and stops working with no agent running. The dividing
line is whether the thing has a right answer.

## Consequences

- A surface is external input: parsed, version-pinned, and validated. A component the
  subset does not know is skipped with a reason rather than guessed at, and a surface is
  data, never instructions to the manager.
- The subset is a floor, not a ceiling. A surface using a component we do not render still
  parses, so Fractory and the skills can write richer surfaces than this build draws, and
  adopting `a2tea` later makes them appear without their authors changing anything.
- Surfaces travel as files, the way an Automation request and a Run's answer already do.
  The alternative, scraping A2UI out of an agent's reply the way a chat host does, works
  only for the three CLIs that keep a readable transcript; a file works for all of them.
- `Agent Surface` is the term, never `Surface`: `internal/ui/surface` already owns that
  word for the cards on the stack.
- Two repositories write surfaces and one renders them, so the format is a cross-language
  contract like the Run record: Fractory validates with `zod`, the Go side mirrors it
  pinned to a version.
