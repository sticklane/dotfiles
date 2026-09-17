# Workspace lifecycle on this Mac

<!-- jj-workflow:start -->
## Default change management — Jujutsu (`jj`)

Use `jj` with Git as the backend for local development. This replaces older Git
staging/commit/pull/push, Worktrunk creation, and Git-hook assumptions below where
jj is enabled. Check for a repository's explicit compatibility exception first.
Read `~/.config/jj/WORKFLOW.md` for commands, validation, setup, and recovery.
Use `jj status`, `jj diff`, `jj log`, `jj new`, `jj file track <paths>`, and
`jj commit -m "type: subject"`; there is no staging area. New files require explicit
tracking on this Mac. Never include another task's work or publish automatically.
Run project checks explicitly: jj does not execute Git hooks, auto-push hooks, or
custom .gitattributes merge drivers. Fetch with `jj git fetch`; push only the named
bookmark with `jj git push --bookmark <task>` after checks and a dry run, according
to existing repo/user publication permissions. Do not advance main implicitly.
For a jj repository, `dev-workspace start <codex|claude|gemini> <task>` creates a
native jj workspace with leases and APFS caches. `dev-workspace create <task>`
creates one without launching an agent. Resume by path with `dev-workspace run`.
These native workspaces are pinned and retained for explicit jj-aware cleanup;
`dev-workspace finish` deliberately refuses Git-only removal. Never force removal.
Worktrunk remains the lifecycle backend for Git-only repositories and existing Git
worktrees. Do not run `wt merge`/`wt step commit` for jj change management.
Fooszone (Git LFS), SDD Harness (frozen Git runtime), and existing linked Git worktrees
are explicit exceptions. Do not initialize jj in them. The bare home dotfiles repo
is accessed through `~/dotfiles-jj`, never by creating `.jj` at `$HOME`.
Git-only tools may use Git internally; direct Git mutations require a concrete
compatibility reason. Never alias git to jj or rewrite another agent's history.
<!-- jj-workflow:end -->

For new isolated CLI tasks, use `dev-workspace start codex <branch>` from the repository.
Worktrunk is the shared worktree manager for Codex, Claude Code, and Gemini.
New Worktrunk workspaces live under `/Volumes/dev-workspaces/worktrees`; this replaces
older temporary-directory and external-drive placement conventions for NEW workspaces.
Existing worktrees, including `/Volumes/fooszone-eval-worktrees`, remain under their
existing owners and explicit cleanup procedures.

Inside a managed worktree, run builds and scratch commands through
`dev-workspace run <command> [arguments]` so caches and temporary files use the APFS pool
and a process lease protects the work. Avoid separate per-run caches and direct exFAT
build directories. The launcher already wraps the agent in a lease.

When all work is committed and preserved on a remote-tracking ref or merged into main,
use `dev-workspace finish` to opt into cleanup after 24 hours. Use `dev-workspace pin`
for intentional retention. Never force removal or bypass lifecycle hooks. Do not
interpret an idle agent marker as permission to delete a worktree.

Codex desktop managed worktrees retain Codex's lifecycle and snapshot/restore behavior.
To use Worktrunk ownership in the desktop app, open its checkout as a Local project.
Do not sweep Codex-managed or legacy directories with this service.

Policy and recovery: `~/.local/share/dev-workspaces/README.md`.
