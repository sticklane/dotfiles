# Workspace lifecycle on this Mac

Use jj with its Git backend; read `~/.config/jj/WORKFLOW.md` before VCS changes.
Use `~/dotfiles-jj` for home dotfiles, explicitly install intended files, and never
initialize jj at `$HOME`. Track new files and run checks explicitly; jj runs no hooks.

For isolation use `dev-workspace create <task>` (or `start codex|claude|gemini <task>`)
from the repository. Check `dev-workspace status` and reuse the registered path for
the same task before creating another. In managed workspaces run builds/scratch via
`dev-workspace run`. New workspaces use `/Volumes/dev-workspaces/worktrees` and shared
APFS caches. Native jj workspaces start unfinished; Git worktrees use Worktrunk hooks.
When the task is complete, review tracked, untracked, and ignored files, preserve all
authored work remotely, and explicitly run `dev-workspace finish`. This queues guarded
cleanup after 24 hours; an empty jj working change requires a remotely preserved parent.
Use `pin` for intentional retention. Legacy jj workspaces remain pinned until reviewed
and explicitly unpinned. Never force removal, infer completion from idle time, or bypass
identity, preservation, lease, and open-file checks. Preserve Codex desktop, legacy,
and existing worktree ownership. Use the `managed-workspaces` skill for lifecycle work.
Policy and recovery: `~/.local/share/dev-workspaces/README.md`.
