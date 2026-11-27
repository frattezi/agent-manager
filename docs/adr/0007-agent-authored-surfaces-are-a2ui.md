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
is the obvious third option, and the reason it is not taken is not the one first recorded
here.

`a2tea` was first deferred on the belief that it required migrating this fork to Bubble
Tea v2. That was wrong. It builds on `charm.land/bubbletea/v2`, a different module path
from this fork's `github.com/charmbracelet/bubbletea`, so Go treats them as unrelated
modules and both can sit in one `go.mod` with no migration at all. Two real constraints
replace that one:

- **One Bubble Tea program per terminal.** A program takes raw mode, owns stdin and
  repaints the screen, so a v2 program cannot run inside this v1 program's frame. It can
  only run in a process of its own — a pane the manager opens, the way a log tab and a
  terminal tab already are. That is a sound design, and it is how `a2tea` would be adopted
  if it were adopted; it is not a reason against it.
- **`a2tea` renders read-only surfaces.** Its own package documentation lists data-model
  bindings, editable inputs and surface lifecycle as gaps, against "styled text, bordered
  cards, container layout, and focusable buttons" that work. Every surface this product
  publishes is a form: a grilling round is choices bound to a data model, check boxes and
  text fields. Pointed at one today, `a2tea` would draw a handsomer card that cannot be
  answered.

So the contract is the **format**, not the library, and we render it ourselves. A surface
is not exempt from looking like the rest of the manager: themed border, a title that reads
as one, muted secondary text, and the focused control filled rather than marked in a
gutter. `a2tea`'s own example shows the bar, and lipgloss v1 clears it — nothing in that
picture needs v2.

`a2tea`'s standing is therefore not "the renderer we will swap to", but "the renderer for
hosts that are not us". A chat host, `crush`, or any CLI that wants to draw our surfaces
can, because the surfaces are A2UI on disk and need nothing from this repository. That is
the payoff of choosing the format, and it is already banked. Should `a2tea` grow bindings
and editable inputs, adopting it becomes a trade of working, themed, matrix-tested code for
a third party's, with its parser attached — worth making only if it is clearly better, not
merely equal.

Deterministic views stay plain Go. The Runs rail, the Tickets view and Automations are
computed from files on disk: a view a model repaints costs tokens for what code decides,
cannot be asserted on its output, and stops working with no agent running. The dividing
line is whether the thing has a right answer.

## Consequences

- A surface is external input: parsed, version-pinned, and validated. A component the
  subset does not know is skipped with a reason rather than guessed at, and a surface is
  data, never instructions to the manager.
- The subset is a floor, not a ceiling. A surface using a component we do not render still
  parses, so Fractory and the skills can write richer surfaces than this build draws.
- Rendering quality is ours to own, not something a later dependency fixes. A surface that
  looks unfinished is a defect in this repository.
- An agent cannot use what it has not been told about. The capability is only real once the
  MCP server's instructions, the tool descriptions and the skills that ask questions all
  say that a question can be a form.
- Surfaces travel as files, the way an Automation request and a Run's answer already do.
  The alternative, scraping A2UI out of an agent's reply the way a chat host does, works
  only for the three CLIs that keep a readable transcript; a file works for all of them.
- `Agent Surface` is the term, never `Surface`: `internal/ui/surface` already owns that
  word for the cards on the stack.
- Two repositories write surfaces and one renders them, so the format is a cross-language
  contract like the Run record: Fractory validates with `zod`, the Go side mirrors it
  pinned to a version.
