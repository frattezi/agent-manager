---
status: accepted
---

# Perceived input lag is a render-path defect, not a Go problem

Interactive lag in the manager raised the question of rewriting it in Rust. Profiling
the frame path first showed a fixed cost of roughly 1.09 ms and 5,866 allocations per
composed Frame, of which about 56% is one redundant render: the footer is composed
twice, once in full purely to count its rows and then discarded, and the footer is the
most expensive element in the Frame (59% of its allocations). We are staying on Go and
removing the redundancy and its amplifier.

## Considered Options

A Rust rewrite was the alternative, and it was rejected on the measurement rather than
on taste. The cost is near-constant in terminal size (0.96 ms at 80x24, 1.35 ms at
400x100), so it is not the renderer being asked to paint too many cells; it is the same
expensive work done twice per message. A rewrite would cut the constant and drop the
garbage-collection term, but it would carry the duplicated work across into roughly
200k rewritten lines, and a Frame composed once already fits the per-message budget
with room to spare.

## Consequences

Lag is therefore a function of message *rate*, not of frame cost: around 1 ms per
message is imperceptible until something floods the event stream. All-motion mouse
tracking, which only engages in Focus mode or with a row menu open, is that flood, and
it is the first thing to look at if this returns.

This ADR covers the Frame path only. The Focus mode input round trip was not measured
here and remains the open suspect for lag felt specifically while typing; a reader
chasing that should measure it against live Sessions rather than assume this ADR
settled it.
