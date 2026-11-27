# Spec 0008: An Agent Surface looks like the rest of the manager

Status: accepted (2026-11-27). Vocabulary: [GLOSSARY.md](../../GLOSSARY.md).

## Goal

An Agent Surface is the one screen an agent authors, and today it is the one screen that
looks unfinished. The card the manager draws around it is themed and respects Compatible
mode; everything inside it is undifferentiated plain text. A round's title, its questions,
the choice the owner is standing on and the choice they are not all render identically, in
the terminal's default colour, inside a border whose corners are the Compatible-mode `+`
even on a terminal that draws box characters perfectly well.

The owner wants a surface to read like the rest of the manager at a glance: a title that
reads as a title, secondary text that recedes, the focused control filled rather than
marked with a character in a gutter, and a border that matches every other card on the
stack. The bar is a card that someone would not guess had been described by an agent rather
than designed. Reaching it needs no new dependency: the theme already carries the roles and
`ChromeSet` already carries the glyphs.

## Non-goals

- **Adopting `a2tea`.** [ADR 0007](../adr/0007-agent-authored-surfaces-are-a2ui.md) records
  why: it renders read-only surfaces, and every surface here is a form. Its role is
  renderers for hosts that are not us. This work is not a stopgap waiting to be replaced.
- **Growing the subset.** The ten components we draw stay the ten components we draw. A
  component outside it keeps rendering as a labelled gap.
- **Changing the A2UI contract.** No new fields, no new version, no change to what an agent
  writes. Every surface already published renders better without being republished.
- **Styling the shell.** `agent-manager surface show` keeps writing plain text, so it stays
  pipeable, diffable and readable in a file.
- **A theme of its own.** Surfaces use the manager's existing roles. There is no surface
  palette, no per-surface colour, and nothing an agent can say that chooses a colour.

## Solution

A surface drawn in the card gains the manager's own visual language.

A **title** reads as one: the `Text` variants `h1`, `h2` and `h3` take the heading roles the
rest of the product uses, rather than rendering as body text. **Secondary text** recedes
into the muted role, so a round's preamble sits behind its questions instead of competing
with them. A **`Button`** with the `primary` variant takes the accent; other buttons stay
outlined.

The **focused control is filled**, the way a selected row is filled everywhere else in the
manager. The `›` gutter mark goes: it was standing in for a highlight the renderer could not
express. Within a focused `ChoicePicker`, the option under the cursor is distinguished from
the option that is chosen — today both are `(*)` and `( )` with nothing to separate "what
you have selected" from "what you are standing on".

The **border follows the glyph mode**, which means in the ordinary case it gains rounded
corners like every other card, and in Compatible mode it becomes consistently ASCII instead
of mixing `+` corners with box-drawing edges. Every glyph the renderer draws — the border,
the rule, the choice markers, the check box — comes from the same `ChromeSet` the rest of
the product reads, so a terminal that cannot draw them gets a surface that still works.

Two things about the card itself are fixed here, because a form that looks right and
behaves wrong is not done:

**Focus brings the whole control into view.** Moving to a field scrolls until that field is
entirely visible, its input line included. Today the viewport stops at the field's first
line, so a `TextField` focused at the bottom of a long round leaves the owner typing into an
input they cannot see.

**Focus wraps.** `tab` from the last control moves to the first, and `shift+tab` from the
first moves to the last. Today the movement clamps, so `tab` at the bottom of a round does
nothing at all while `shift+tab` works, which reads as a broken key.

Nothing about publishing, answering, retention or the round trip changes. The same
documents, the same keys, the same answers.

## User stories

1. As the owner, I want a round's title to read as a title, so that I can tell at a glance
   what I am being asked about.
2. As the owner, I want a round's preamble to recede, so that the questions are what I read
   first.
3. As the owner, I want the control I am standing on to be filled, so that I never lose my
   place in a long round.
4. As the owner, I want the option under my cursor distinguished from the option that is
   selected, so that I can tell what `space` or `↵` is about to do.
5. As the owner, I want the submit button to carry the accent, so that the way out of the
   card is obvious.
6. As the owner, I want a surface's border to match every other card on the stack, so that
   the manager looks like one program.
7. As the owner on a terminal without box-drawing characters, I want the whole surface drawn
   in ASCII consistently, so that it degrades rather than breaking.
8. As the owner in Compatible mode, I want the choice markers, check boxes and rules to come
   from the same set as everything else, so that one setting governs the whole screen.
9. As the owner, I want my theme's colours used, so that a surface follows a light or dark
   terminal the way the rest of the manager does.
10. As the owner who changes theme while a card is open, I want the surface repainted in the
    new theme, so that nothing is left stale behind the modal.
11. As the owner, I want the field I move to brought fully into view, so that I can see what
    I am typing.
12. As the owner, I want a field taller than the card to show its input even so, so that a
    long question is still answerable.
13. As the owner, I want `tab` from the last control to wrap to the first, so that a key
    that is advertised always does something.
14. As the owner, I want `shift+tab` from the first control to wrap to the last, so that
    movement is symmetrical.
15. As the owner, I want the scroll indicators to keep telling me there is more above or
    below, so that wrapping does not hide the shape of the round.
16. As the owner using the mouse, I want clicking a control to focus it and bring it fully
    into view, exactly as `tab` does, so that both input methods land in the same state.
17. As the owner, I want the card's footer to keep naming the keys for the control I am on,
    so that the hints stay true as focus moves.
18. As an agent, I want my existing surfaces to render better with no change to what I
    publish, so that the improvement is free.
19. As an agent, I want a component outside the subset to keep rendering as a labelled gap,
    so that a richer surface degrades rather than failing.
20. As an agent reading a surface back from a shell, I want `surface show` to stay plain
    text, so that I can pipe it, diff it and read it in a log.
21. As a maintainer, I want the role of each piece of text asserted in tests rather than its
    colour, so that the suite does not break when a palette changes.
22. As a maintainer, I want the renderer to stay pure, so that it is tested without a store,
    a terminal or a theme loaded.
23. As a maintainer, I want one place that maps A2UI variants to roles, so that adding a
    variant is a table entry rather than a hunt.
24. As a maintainer, I want layout still asserted on plain text, so that a styling change
    cannot silently move the furniture.

## Implementation decisions

**The renderer emits roles, not colours.** A rendered line stops being a string and becomes
an ordered list of segments, each carrying its text and a **role** — a small enumeration
owned by `internal/ui/a2ui`, naming what a piece of text *is* (title, body, muted, label,
value, accent, focused, selected, gap) rather than how it looks. The package gains no
dependency on the theme or on lipgloss: it says what the text means, and the caller decides
what that looks like. Segments rather than one role per line, so a label and its value
differ on the same row.

**The card owns the mapping.** `internal/ui/surfaces`, which already reads the theme for the
window chrome, supplies a table from role to style and applies it when it draws. That is the
single place A2UI variants become colours, and it is a table, not a decision made per
surface. The existing theme roles are the targets; this Spec adds no new ones.

**Glyphs come from `ChromeSet`.** The renderer takes the already-resolved set rather than
reading a setting or hard-coding characters. Today's card hard-codes the Compatible-mode
corner glyph in every mode, which is the whole of the mixed-border defect; passing the set
in makes the right glyphs arrive and removes the special case.

**Plain text is styled text with an empty table.** `Render`, which `surface show` uses,
flattens segments and ignores roles. One renderer, two presentations, so the shell output
and the card can never disagree about layout.

**Focus is a range, not a point.** The card's focus model records the first and last line of
the focused control, and the viewport scrolls until both are inside it, preferring to show
the last line when a control is taller than the card so the input stays visible. Movement
wraps modulo the control count instead of clamping.

**Boundaries.** `internal/ui/a2ui` keeps its existing boundary: no store, tmux, Git or file
access, and now explicitly no theme and no lipgloss either, which is stricter than the
alternative of letting it import them. `internal/ui/surfaces` already imports the theme, so
its boundary entry in the architecture check is unchanged.

## Testing decisions

One seam carries almost all of it, and it is the one that already exists: the renderer's two
entry points are pure functions from a parsed document and a width to output. Tests assert
on the **segment model** — which text carries which role, in what order — never on ANSI
bytes. A palette change cannot break them, and a reader can see what a test means.

- `internal/ui/a2ui` is tested on roles and targets: variants map to the roles expected, the
  focused control carries the focused role, the cursor option is distinguished from the
  selected one, the glyphs come from the set passed in, and a component outside the subset
  still yields its labelled gap. The existing golden-text fixtures stay, rendered with roles
  flattened, so layout remains asserted as plain text and a styling change cannot quietly
  move the furniture. This is the prior art the package already has.
- `internal/ui/surfaces` is tested on the card's behaviour: moving focus wraps at both ends,
  a focused control is brought entirely into view, a control taller than the card still
  shows its input, a click focuses and scrolls the same as a key, and the scroll indicators
  stay honest. The existing card tests are the pattern.
- Evidence against a real session is a captured frame, not a green suite: the throwaway
  `HOME` recipe in CONTRIBUTING.md, a real surface published from a real pane, frames read
  back in both glyph modes and in a light and a dark theme. That is the standard this
  feature was held to when it was demonstrated, and it is how the owner will check it.

The matrix applies unchanged: the keys named here are already in the footer and under `?`,
and wrapping adds no key, so no new entry is needed — but the footer must keep naming the
right keys as focus moves, which is a test, not a note.

## Open questions

- Whether a focused `TextField` should show a cursor within its input, or keep the current
  underscore run. Visual polish, decidable once the roles land.
- Whether `surface show` should gain an opt-in flag to emit styled output for a terminal
  that wants it. Nothing needs it today, and plain text is the right default.
- Whether a very long round should gain a per-question jump, rather than only `tab`. Only
  worth asking once rounds are routinely longer than a card.
