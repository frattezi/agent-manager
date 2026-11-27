# 8. A Bot's context is configuration, not a first prompt

Date: 2026-11-27

## Status

Accepted. Supersedes the delivery mechanism in
[Spec 0005](../specs/0005-bots.md); the storage decision in
[ADR 0006](0006-bots-configured-here-knowledge-versioned-in-fractory.md) stands.

## Context

Bots shipped specializing a session with a first prompt. The spawn assembled a
message and passed it as the launch command's positional argument:

```
You are a Bot started from the Profile "orchestrator".
Load the fractory-orchestrator skill now and work as it says for the rest of this conversation.
Your memory is the ai-memory project "...": recall from it before you answer anything.
Then say in one line that you are ready, and wait for the owner to ask.
```

It worked, in the sense that the Bot usually complied. But a first prompt is a
user turn, and a user turn has four failure modes that configuration does not:

- **It decays.** The specialization is early context, so compaction can drop it.
  A Bot talked to for an hour is less specialized than one just opened, and
  nothing says when it stopped being specialized.
- **It is unverifiable.** Nothing checks the skill was found, let alone followed.
  The Profile named `fractory-orchestrator` for a week while no such skill
  existed; every Bot reported it could not find it and carried on as a plain
  conversation, and the Profile still looked correctly configured.
- **It costs a turn** on every spawn, to tell a model to go and read something
  the host already knows the location of.
- **It is lost on revive.** `AssembleResume` types a prompt only when given one,
  and revive gives none, so a Bot that comes back is not a Bot any more. The
  Profile's launch flags, by contrast, persist in `launch_flags` and do return.

The owner's own No Token Architecture rule names this directly: if a step has a
right answer, it is code with a test, not an instruction to remember. Asking a
model to load its own identity is the instruction case.

## Decision

A Bot's Specialization and memory reach it as configuration, before its first
token, and no part of its identity is delivered as something it is asked to do.

**The Specialization is passed as a system prompt file on the launch command.**
The file is the skill in the Fractory repository — not a copy, not inlined text —
so editing the skill changes what the next Bot is, with no migration and no
release.

**The flag is a field on the tool definition, not a literal in the Bots package.**
`--append-system-prompt-file` is Claude Code's spelling; another CLI spells it
differently or not at all. The capability is declared per tool in
`internal/config/builtin.go`, set for every CLI that has one, and a CLI without
it starts its Bot unspecialized rather than failing. Hardcoding the Claude flag
would break both the Thin Wrapper Principle and the every-tool rule in
[AGENTS.md](../../AGENTS.md).

**The memory project is bound by an `.ai-memory.toml`** in the Bot's own
directory, which ai-memory resolves from the nearest marker. This costs no tokens
at all: the binding is a file, not a sentence, and there is nothing for the model
to comply with.

The marker is written only into a directory the Profile owns, never into a
repository the owner works in. An `.ai-memory.toml` in a real checkout would
re-scope memory for every agent that runs there. A Profile whose directory is a
repository inherits that repository's memory scope instead, which is the right
answer for a Bot working inside it.

The first prompt is removed. A Bot opens on an empty composer, specialized.

## Consequences

A Bot is specialized from token zero, stays specialized through compaction,
returns specialized from a revive, and spends no turn on either. A Profile
pointing at a skill that does not exist now fails visibly at spawn instead of
producing a Bot that looks configured and is not.

The cost is that specializing a Bot now depends on a capability of the CLI it
runs, where a first prompt worked on anything that reads stdin. That is the
honest trade: the previous mechanism worked everywhere and guaranteed nothing.

A skill file is read as a system prompt, so its YAML frontmatter travels with it
unless stripped. Whether to strip it is an implementation question, not a
decision this record settles.

This reverses only the delivery. Where a Specialization lives — a committed,
versioned skill in Fractory rather than text in a configuration field — is
unchanged, and this decision depends on it: the file is the thing we can point a
system prompt flag at.
