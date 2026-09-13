# Workspace lifecycle on this Mac

Use Worktrunk for new worktrees. Its installed Claude plugin routes native worktree
isolation through the same hooks. New paths belong under `/Volumes/dev-workspaces/worktrees`;
older worktrees and the Fooszone eval volume remain legacy resources with their existing owners.

Start an isolated session with `dev-workspace start claude <branch>` from its repo.
For builds and temporary commands in managed worktrees, use
`dev-workspace run <command> [arguments]`. This sets per-workspace scratch and shared
APFS cache locations and holds a process lease. Do not create standalone exFAT caches.

After preserving completed commits and required artifacts, use `dev-workspace finish`
to opt into removal after 24 hours. `dev-workspace pin` retains a workspace explicitly.
Never force removal, disable hooks, or treat idle activity markers as deletion authority.
Native Claude removal is subject to the same preservation and lease checks.

See `~/.local/share/dev-workspaces/README.md` for budgets, status, and recovery.
