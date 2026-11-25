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

## 3. Terminal and chat side by side

Show an agent's chat and one of its nested terminals in two panes at once, instead of switching
between rows. Today the split view is the session list plus a single preview; the workaround is an
outer terminal split running `tmux attach -t am_<name>`. Could share the multi-pane work with idea 2.

## 4. Planning views

Special views for the work before a Run:

- **Grilling:** a `grill-with-docs` session shown as the questions asked, the answers given and the
  decisions it wrote to the glossary and ADRs, so a Goal's progress toward a Spec is visible.
- **Tickets:** the repo's Tickets from GitHub Issues or Jira, with their status (backlog, ready, in
  progress, in review, done), the Spec they came from and the Runs and PRs on each.
