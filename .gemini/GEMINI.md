## Gemini Added Memories
- I installed the 'bv' (Beads Viewer) tool to ~/.local/bin/bv. It requires a beads project (.beads/beads.jsonl) to work.
- I installed 'font-hack-nerd-font' via Homebrew.
- The user is asking about email filters, likely in Gmail.
- My email address is steven.jaconette@gmail.com

## Workspace lifecycle

Use `dev-workspace start gemini <branch>` from a repo for new isolated tasks.
Worktrunk is the shared manager; new workspaces use `/Volumes/dev-workspaces/worktrees`.
Keep existing worktrees under their existing owners. In managed workspaces, use
`dev-workspace run <command> [arguments]` for builds and scratch commands so leases,
temporary directories, and shared APFS caches are applied consistently.
After committing and preserving completed work, use `dev-workspace finish`; cleanup
waits 24 hours and preserves dirty, pinned, locked, or active work. Use
`dev-workspace pin` for intentional retention. Never force removal or bypass hooks.
Read `~/.local/share/dev-workspaces/README.md` for policy and recovery.
