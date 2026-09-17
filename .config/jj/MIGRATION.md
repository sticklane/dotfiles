# Jujutsu migration — 2026-09-16

Installed jj 0.45.1 with Git as backend. Twenty-two ordinary GitHub checkouts are colocated; dotfiles use a dedicated jj workspace backed by the existing bare repository. GitHub remotes, original branches/tags, and pre-existing working content were preserved. No GitHub pushes or deployments were made.

## Configuration and guidance

- Machine config: `~/.config/jj/config.toml`; explicit new-file tracking, existing Steven identity, colocated new clones.
- Workflow: `~/.config/jj/WORKFLOW.md`.
- Updated global AGENTS/CLAUDE/GEMINI files and each inventoried repo's root agent files.
- Repo index generator now shows jj/Git status in `~/REPOS.md`.
- Dotfiles changes are separately preserved at `~/dotfiles-jj`; pre-existing home edits were excluded from that change.

## Repositories

| Path | Result |
|---|---|
| `~/archive/dev-agents` | jj / Git; local migration bookmark |
| `~/archive/terminal-tasks` | jj / Git; local migration bookmark |
| `~/archive/ynab-app` | jj / Git; local migration bookmark |
| `~/archive/clothing` | jj / Git; local migration bookmark |
| `~/archive/claude` | jj / Git; guidance snapshotted alongside existing edits |
| `~/archive/codewalk` | jj / Git; local migration bookmark |
| `~/archive/portfolio-tracker` | jj / Git; guidance snapshotted alongside existing edits |
| `~/archive/decision-dash` | jj / Git; local migration bookmark |
| `~/archive/tdd-git-hooks` | jj / Git; local migration bookmark |
| `~/archive/tasks-app` | jj / Git; local migration bookmark |
| `~/specs` | jj / Git; local migration bookmark |
| `~/trmnl-stocks-colour` | jj / Git; local migration bookmark |
| `~/portfolio-tracker` | jj / Git; local migration bookmark |
| `~/wft-player-watch` | jj / Git; local migration bookmark |
| `~/interview-prep` | jj / Git; local migration bookmark |
| `~/budget_analysis` | jj / Git; local migration bookmark |
| `~/vaults/life` | jj / Git; local migration bookmark |
| `~/fooszone` | Git exception: Git LFS |
| `~/trmnl-calendar-color` | jj / Git; local migration bookmark |
| `~/automation` | jj / Git; local migration bookmark |
| `~/sdd-harness` | Git exception: frozen Git-based runtime |
| `~/ynab-mcp-new` | jj / Git; local migration bookmark |
| `~/hub` | jj / Git; guidance snapshotted alongside existing edits |
| `~/trmnl-notify` | jj / Git; local migration bookmark |
| `~/dotfiles-jj` | jj; existing `~/.dotfiles.git` backend |

## Preservation and limitations

- 19 initially clean repos have local documentation commits on `chore/jj-migration-20260916`; main was not moved. The 3 initially dirty repos retain their changes and the new guidance in jj working snapshots. The 2 Git exceptions have local guidance edits.
- Existing linked Git worktrees (122 registered paths across the inventory, including primary checkouts) keep their owners. Native jj cannot be colocated into linked Git worktrees.
- Native jj workspaces use the existing admission, APFS caches, and leases, but are pinned/retained. Git-only cleanup explicitly refuses them. Automated jj cleanup is not implemented.
- The live registry has reached its existing 12-workspace limit. A live `dev-workspace create chore/jj-migration` attempt correctly refused creation. No occupied workspaces were removed or limits bypassed.
- Git LFS is unsupported by jj, so Fooszone remains Git-managed. SDD Harness keeps its frozen Git runtime.

## Verification

- Checked tracked-file hashes before and immediately after initialization; no changes.
- Verified all original branch and tag refs after initialization and guidance updates.
- All 22 jj status checks passed; the generated index contains 22 jj rows.
- Workspace Go test suite, go vet, gofmt, and launchd plist validation passed.
- New tests cover jj creation, managed command execution, leases, identity tampering, duplicate/unsafe names, insufficient storage, and refusal by Git-only cleanup.
- Tested and confirmed linked-worktree initialization is rejected by installed jj.

## Recovery

`~/.local/state/jj-migration/` contains inventory, verification, local-commit records, and private backups of agent files, Git refs, HEAD/index/config, and staged/unstaged patches. Keep this directory local. Use `jj op log` to inspect operations before recovery; do not overwrite another session's changes.

Official compatibility reference: https://docs.jj-vcs.dev/latest/git-compatibility/
