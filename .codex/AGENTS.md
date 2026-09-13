# Workspace lifecycle on this Mac

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
