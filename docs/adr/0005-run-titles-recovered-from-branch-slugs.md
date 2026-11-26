# Run Titles are recovered from branch slugs until Fractory records them

A Run record carries no Run Title. The manager needs one, because a Runs rail row that
shows the raw Ticket Ref spends its width on `github:frattezi/`, constant for every Run a
single owner has, and tells the reader nothing that distinguishes one row from the next.
The only human text on the record is the slug inside the Run's branch, so the rail
reconstructs the Title from it: strip everything up to the last `/`, strip the leading
ticket number, swap hyphens for spaces, capitalise the first letter. When a record one day
carries a `ticketTitle`, the rail prefers it and the reconstruction becomes dead code.

Two alternatives were rejected. Reading titles from the forge would put a network call
behind every row, break the rail offline, and bind the manager to GitHub's API for
something that is already in Fractory's hands. Waiting for Fractory to record the title
would leave the rail unreadable in the meantime, for a field that is one line to add at
the source. Reconstructing it is wrong in a visible, harmless way — the reader sees a
title that reads slightly oddly, not a title that is missing.

## Consequences

- The reconstruction is lossy at source and stays that way. An apostrophe reached the
  branch as `-s-` and comes back as ` s `; Fractory truncates long slugs mid-word, so a
  Title can end on a fragment. No rule is added to repair either, because every such rule
  mangles words that were genuinely hyphenated.
- The de-slug is one pure function with a table test, not a step an agent is asked to
  remember.
- `frattezi/fractory` holds the Ticket for writing `ticketTitle` into the Run record. That
  Ticket is this ADR's expiry date.
- The Ticket Ref stays on the record, in the detail pane, in the search match text and as
  the name of a Review target. Only the surfaces a human reads for orientation — the rail
  row and its menu — show the Title instead.
