# Fractory Manager ideas

Ideas for this fork (Fractory Manager, the Agent Manager fork that shows Fractory Runs). Not specs
yet: each goes through grilling and a Spec before any work. Factory-side ideas live in
`frattezi/fractory`, `docs/ideas.md`.

## 1. Draft review links after a PR review

When a PR-review task finishes, Fractory Manager shows a list of the draft reviews it produced, with
their URLs, so the owner can open and submit them. At that point the review agent ends itself and
its worktree is cleaned up: nothing stays running or on disk once the drafts exist.

## 2. A live map of the factory

A TUI view of the agents that are working right now and how they connect: which Runs belong to
which Ticket and Spec, which agent and account each uses, and which Triggers started them. A large
visual piece that needs its own design pass. The header already reads Fractory Manager; the binary
is built as `fractory-manager`.

## 3. Tile the screen: regions, jumps and a launcher

Manage the manager's screen the way i3 manages a desktop. The screen stops being a fixed layout the
manager draws and becomes a tree of regions the owner arranges, each holding a session pane, a
terminal, or one of the manager's own views.

- **Regions side by side.** An agent's pane and a terminal visible at once, instead of switching
  rows. Today the layout is the session list plus a single preview; the workaround is an outer
  terminal split running `tmux attach -t am_<name>`.
- **Jump by number.** One chord per region, so moving between what is on screen takes one key and
  no selection walk.
- **A launcher.** A palette, in the spirit of Spotlight, that opens a new terminal or agent session
  on the current project *beside* the conversation the owner is in, rather than as a row to find
  later.

The manager composes its own frame in Go and never asks tmux to split, so the region tree is the
manager's to build. Idea 2 needs the same tree to place its map. Specced as
`docs/specs/0001-screen-regions.md` on the `feat/screen-regions` branch.

## 4. Planning views

Special views for the work before a Run:

- **Grilling:** a `grill-with-docs` session shown as the questions asked, the answers given and the
  decisions it wrote to the glossary and ADRs, so a Goal's progress toward a Spec is visible.
- **Tickets:** the repo's Tickets from GitHub Issues or Jira, with their status (backlog, ready, in
  progress, in review, done), the Spec they came from and the Runs and PRs on each.
