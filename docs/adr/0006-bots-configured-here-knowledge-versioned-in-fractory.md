# Bots are configured in the manager and know what Fractory commits

A Bot Profile is an identity plus the choices that start an agent from it: the CLI and model,
how it is shown, where it remembers, and which Specialization it loads. The manager owns that
configuration in `state.db`, lists it in a Bots area, and spawns a Bot from it as an ordinary
Session the owner can attach to. The Specialization itself is not in that store: it is a
committed skill in the Fractory repository, which the Profile names. Fractory executes nothing
for a Bot.

The obvious design is the opposite one, and we started there. Fractory is a config package
(its ADR 0001) holding `rules/`, `flows/` and `skills/`, so a `bots/` directory beside them
looks like the house pattern, and Automations had just set the precedent of the manager holding
configuration for Fractory to execute (ADR 0004). Two things moved it. A Bot is a Session, and
Sessions are the one thing the manager owns outright — routing a spawn through a file Fractory
reads would buy nothing and lose the picker, Settings and the CLI. And a Profile is exactly the
kind of value this project refuses to put in a hand-edited file: a setting the owner changes
belongs on a Settings row, in `state.db`, reachable from `agent-manager bots`.

Splitting the two is the point rather than a compromise. What the owner tweaks — a colour, an
emoji, a model — is cheap to lose and wants a picker. What a Bot knows is code: it earns review,
history and a diff, and it is the part another machine needs to behave the same way.

## Consequences

- A Bot runs whatever CLI the Profile names, not Claude alone, because a Bot is a Session and
  the manager already launches every tool in `builtinTools`.
- A Profile is not version-controlled. A lost `state.db` loses the colour and the model choice;
  it never loses the Specialization, which is in git.
- The Specialization is a skill, so it costs no context until a Bot loads it, and the same skill
  serves a Bot, an ordinary Session and a Run without being written twice.
- A Bot's usefulness is bounded by the commands its Specialization can call. Anything with a
  right answer belongs in `fractory doctor`, `fractory limits` or `fractory tokens`, not in the
  Profile's prompt.
- Nothing is scheduled. A Bot is conversational: it acts when the owner opens one and asks.
  Reacting to a state change without being asked stays with Fractory's Triggers and Runs.
