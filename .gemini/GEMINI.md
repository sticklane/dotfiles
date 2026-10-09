## Gemini Added Memories
- I installed the 'bv' (Beads Viewer) tool to ~/.local/bin/bv. It requires a beads project (.beads/beads.jsonl) to work.
- I installed 'font-hack-nerd-font' via Homebrew.
- The user is asking about email filters, likely in Gmail.
- My email address is steven.jaconette@gmail.com

## Workspace lifecycle

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
