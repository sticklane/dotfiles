# Local Jujutsu workflow

Jujutsu 0.45.1 is the default change-management CLI. Git remains the object store,
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
# when publishing is authorized and the preview is correct:
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

Fetch does not merge or rebase your work. Inspect `jj log` after fetching and rebase
only your own unpublished changes when needed (`jj rebase -s <change> -d <base>`).
Do not rewrite published/shared changes or move main merely to silence divergence.
Keep existing repo/user publication permissions; migration itself does not authorize
publishing unrelated work. No migration commits have been pushed to GitHub.

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
Native workspaces have no `.git` directory; software that requires a Git working
tree needs an explicitly evaluated compatibility route. Metadata queries can use
`jj git root` to locate the backing store. Do not fabricate `.git` links/indexes.

Native jj workspaces are pinned and retained. `finish`/the collector deliberately
refuse them: Git-only cleanliness checks cannot establish preservation of jj state.
Cleanup must be explicitly reviewed for anonymous changes, remote preservation,
untracked and ignored data, workspace identity, and live processes. Unpinning is not
an override. This is a conservative retention boundary, not completed automated jj
cleanup support. Worktrunk and its guarded cleanup continue for Git-only worktrees.
Existing Codex-managed, legacy, and active linked Git worktrees are not converted.

## Exceptions

- `~/fooszone`: active Git LFS paths; jj does not support LFS.
- `~/sdd-harness`: frozen Git-based harness runtime/candidates. Do not alter its backend.
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
