# A Bot's context is configuration, not a first prompt

A Bot used to learn its Specialization from its first user turn: "Load the <name> skill now and
work as it says for the rest of this conversation." That line spends a turn, fades when the
conversation is compacted, is gone when the session is revived, and cannot be checked from
outside. The `orchestrator` Profile named a skill that did not exist for a week, and every Bot
from it opened as a plain conversation without anyone noticing.

The Specialization now travels as configuration. Each tool definition declares the flag that
appends a file to its system prompt (`append_system_prompt_flag`). At spawn the manager resolves
the Profile's Specialization to a `SKILL.md`, first in the Fractory repository's
`.agents/skills/<name>/`, then in the global `~/.claude/skills/<name>/`, and appends the flag and
the path to the Profile's own flags. Those are the session's launch flags, stored in
`launch_flags`, so a revive launches with the same file.

## Consequences

- A Specialization that resolves to no file is an error at spawn and starts nothing. A Bot
  never opens looking specialized when it is not.
- The file is passed by path and never read, inlined or copied into `state.db`. Editing the
  skill in git changes what the next Bot, and the next revive, is given.
- The skill's YAML frontmatter is not stripped. Stripping it means writing a copy somewhere,
  which would stop following the file in git and could be gone before a revive; the frontmatter
  is a name and a description, harmless as text in a system prompt.
- On a CLI without the flag a Bot starts as an ordinary session with no Specialization. Adding
  the flag to that tool's definition is what brings it in.
- The ai-memory line still rides the first prompt; it moves the same way separately.
