
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
