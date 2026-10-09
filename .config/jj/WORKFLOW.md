# Local Jujutsu workflow

Jujutsu 0.46.0 is the default change-management CLI. Git remains the object store,
GitHub transport, and compatibility backend. There is no `git` shell alias.

## Daily work

Run these from the repository or native jj workspace root:

```sh
jj status
jj diff
jj log --limit 15
jj new                         # start a separate change; don't absorb unrelated work
jj file track path/to/new-file # explicit new-file tracking on this Mac
# edit and run the repository's checks
jj commit -m 'feat: describe the change'
jj bookmark set task/example -r @-
jj git fetch --remote origin
jj git push --remote origin --bookmark task/example --dry-run
# when the checks pass and the preview is correct:
jj git push --remote origin --bookmark task/example
```

`jj commit` finishes the current change and opens an empty child. Bookmarks do not
advance automatically: name the finished revision explicitly. `@` is the working
change; `@-` is its parent. Use explicit file arguments to `jj commit` when other
work exists. Review every selected diff; already tracked files snapshot automatically.
User config sets `snapshot.auto-track = "none()"` so new files are never silently
included. Existing untracked files stay untracked. Do not use a blanket file-track
command in a shared checkout. With `-R`, file arguments are still relative to your
shell directory: change directory first or use a `root:` fileset.

A requested change includes publishing it. When it is finished and the checks
pass, point `main` (or the repo's documented integration bookmark) at it and push;
no separate publish approval is needed. Ask first only when the push would carry
another agent's unpublished work, rewrite shared history, or the request said not
to push.

Fetch does not merge or rebase your work. Inspect `jj log` after fetching and rebase
only your own unpublished changes when needed (`jj rebase -s <change> -d <base>`).
Do not rewrite published/shared changes or move main merely to silence divergence.
Migration itself does not authorize publishing unrelated work. No migration commits
have been pushed to GitHub.

## Checks and integrations

jj does not execute Git hooks. Run the repo's documented lint, formatting, test,
typecheck, and review gates explicitly before finalizing/publishing changes.
Where provided, `bash scripts/check.sh` is the canonical full check. Git post-commit
auto-push no longer applies to jj commits. Beads Dolt sync remains a separate explicit
operation; jj does not run the custom Beads `.gitattributes` JSONL merge driver.
Git-only integrations can use Git internally, but agents use jj for authored changes.
Read-only Git compatibility queries are allowed. Do not bulk-rewrite programs that
implement Git backends into guessed jj equivalents.

## Isolation and storage

```sh
dev-workspace start codex task/example   # also claude or gemini
dev-workspace create task/example        # just create; outputs the path
cd /absolute/path/from/output
dev-workspace run <command> [arguments]
```

In a jj repo the launcher creates a native jj workspace from the current change's
parent (`@-`), under `/Volumes/dev-workspaces/worktrees`. Review the intended base
first. It shares the Git object store and jj history, has its own working change,
and gets the existing APFS cache, admission, identity, and process-lease protections.
Resume by path with `dev-workspace run`; do not create the same task again.
Creation explicitly uses `--no-colocate` even when `git.colocate=true`.
Native workspaces have no `.git` directory; software that requires a Git working
tree needs an explicitly evaluated compatibility route. Metadata queries can use
`jj git root` to locate the backing store. Do not fabricate `.git` links/indexes.

Native jj workspaces start unfinished and are retained until explicit completion.
Reuse an existing registered task path before creating another workspace. After all
authored work is remotely preserved, review untracked and ignored data and run
`dev-workspace finish <path>`. The collector checks the exact jj revision (or an
undescribed empty child of a remotely preserved parent), workspace identity, shared
repository location, file inventory, leases, and open processes. It waits 24 hours.
Use `pin` for intentional retention. Older jj workspaces remain pinned; explicitly
unpin them only after reviewing their contents and ownership, then finish them.

For user-authorized immediate cleanup of a specific finished native workspace,
preview `dev-workspace remove-jj <path>`, then use the same command with `--apply` from outside that workspace.
This bypasses only the retention delay; every preservation/identity/process check
still applies. It forgets that workspace, never shared commits or operation history.
Removal first quarantines the tree. An interruption retains its registry record
and any remaining quarantine for manual recovery, even if the original path is gone. Use `dev-workspace recover-jj <original-path>` to preview safe cancellation/restore
while the preserved registration exists; `--apply` pins it and resets completion.
Otherwise do not force-delete it, restore shared history, or automatically re-adopt it. Idle time,
task closure, or an empty current change does not authorize deletion. Existing
Codex-managed, legacy, and linked Git worktrees keep their original owners/lifecycle.

## Exceptions

- `~/fooszone` now uses jj; model/evaluation assets live in pinned GCS manifests.
  Existing linked Fooszone Git worktrees retain their original lifecycle.
- `~/sdd-harness` uses jj for development with its colocated Git backend. Existing
  linked Git candidates keep their ownership and history. This does not activate
  the harness runtime's unfinished native jj target-repository adapter.
- Existing linked Git worktrees: jj refuses colocated initialization in them.
- Bare home dotfiles: use `~/dotfiles-jj`, backed by `~/.dotfiles.git`. Never initialize
  `.jj` at `$HOME` (that would make unrelated folders inherit the home repository).
  Edit/review in the dedicated workspace, then explicitly install only intended files
  into `$HOME`. Do not copy its `.jj` metadata. Since it is non-colocated, use
  `jj git import`/`jj git export` when synchronizing changes with the bare Git repo.

## New repositories

Use `jj git clone <GitHub-URL> <directory>` for new clones. For an existing ordinary
checkout use `jj git init --colocate` only after inspecting status, staged work,
Git LFS/filter attributes, submodules, sparse/partial clones, and active operations.
Do not initialize linked worktrees or compatibility exceptions. Keep `.git` and all
remotes. Set per-repository identity where it differs from the default Steven identity.

## Recovery

Inspect `jj op log` and `jj op show <operation>` before deciding to undo an operation.
Do not blindly restore history shared with another active agent. Git hooks and the
original Git repositories were kept. Migration metadata, original index/HEAD/config,
agent-file backups, refs, and patches are at `~/.local/state/jj-migration/backups`.
The initial inventory and results are alongside them. Backups can include private
working content; keep that directory local. The index is regenerated by
`~/.local/bin/repo-index.sh` and shows the VCS in `~/REPOS.md`.

References: https://docs.jj-vcs.dev/latest/git-compatibility/
and https://docs.jj-vcs.dev/latest/working-copy/.
