# Managed development workspaces

Worktrunk owns new Git worktrees. `dev-workspace` supplies admission, process leases,
preservation checks, per-workspace scratch, shared native caches, and crash recovery.
The code uses only the Go standard library. Configuration and source are tracked in
the bare dotfiles repository. Worktrunk itself is installed through Homebrew.

Agent integration installation commands (already run on this machine):

```sh
brew install worktrunk
wt config plugins claude install -y
wt config plugins codex install -y
gemini extensions install https://github.com/max-sixty/worktrunk --ref v0.77.0 --consent
wt config shell install zsh -y
```

## Daily use

From a repository:

```sh
dev-workspace start claude feat/example
dev-workspace start codex feat/example
dev-workspace start gemini feat/example
```

Choose one agent per task. Arguments after `--` are passed to that CLI. The agent's
exit releases its lease; it does **not** mark unfinished work disposable.

To return to a task, use `wt switch <branch>`, then `dev-workspace run codex` (or
Claude/Gemini). Also use `dev-workspace run <command> [arguments]` for builds,
test runs, and temporary commands. It configures shared uv, npm, pip, Go/module,
Playwright, and Puppeteer caches on APFS, plus per-worktree `.dev-build` scratch
and Cargo build output. These environment values propagate to child processes.
Use Cabal's `--builddir=.dev-build/cabal` for Haskell builds.

When the task is done, preserve commits and required artifacts and run
`dev-workspace finish <path>`. An owning agent can mark completion before exiting;
its live lease continues to block removal until it exits. The worktree becomes
eligible after 24 hours. `wt remove <path>` performs guarded immediate removal.
Local and remote branches are preserved. `dev-workspace pin <path>` retains a tree;
`unpin` releases that policy. Unfinished idle trees remain visible until reviewed.

Codex desktop's own managed worktrees keep Codex's snapshot/restore lifecycle.
For Worktrunk ownership in the desktop app, open a Worktrunk checkout as a Local
project. Do not apply this collector to Codex-owned directories.

Claude's Worktrunk plugin routes native WorktreeCreate/WorktreeRemove events through
these hooks. Codex and Gemini have Worktrunk plugins/extensions for guidance and
activity tracking; their common creation entry point is the launcher above.
Markers are informational. A live PID plus its start time protects a leased process.
Raw agent launches and commands using `--no-hooks` bypass parts of this protocol.

## Storage and budgets

The new `/Volumes/dev-workspaces` APFS pool is a **64 GiB maximum sparse image** at
`/Volumes/SSK SSD/development/dev-workspaces.sparsebundle`. Its actual available
capacity is limited by the backing drive. The separate 24 GiB Fooszone image is
unchanged and is never unmounted or cleaned by this service.

The Seagate drive has ample capacity but is currently read-only NTFS. It was not
reformatted or remounted. SSK is therefore a guarded transitional destination.

`~/.config/dev-workspaces/config.json` records both volume UUIDs and the exact
mount points. A missing, wrong, suffixed, or read-only mount fails closed; there
is no internal-disk fallback. Worktrunk path settings live in
`~/.config/worktrunk/config.toml` and include repository and branch identity.

Every new workspace reserves 4 GiB. Admission serializes reservations, includes
in-flight creations, and requires 8 GiB free in the pool, 12 GiB on SSK, and 2 GiB
internally **after** reservations where relevant. Maximum registry count is 12;
current backing-drive headroom permits far fewer. These are admission checks,
not filesystem quotas: an already running build or unrelated application can
still exhaust a volume. Recovering roughly 25 GiB of internal headroom remains
a separate legacy-cleanup priority; the 2 GiB hard floor is transitional.

Installed dependencies and `.dev-build` scratch disappear with their worktree.
The only globally declared disposable ignored directories are `.dev-build`,
`node_modules`, `.venv`, `__pycache__`, `.pytest_cache`, `.mypy_cache`, and
`.ruff_cache`. Other ignored files, including `.env`, local databases, `dist`,
and `target`, block removal until their owner deliberately preserves or removes
them. Do not hide valuable state in declared disposable directories.

Shared uv/npm/Go result caches target 2 GiB each. Daily maintenance first uses
`uv cache prune`, then native full-cache eviction if an idle cache exceeds its
target. These are soft targets checked during maintenance, not hard quotas.
Package/module and browser stores share the pool's overall admission budget;
they are retained rather than generically deleted. No existing cache directory
outside the new pool is cleaned. Native tools own cache format and locking.

Sparse-image deletion does not necessarily return space to SSK immediately.
After all managed trees are released, the service can safely detach, compact,
and reattach **only the new image**. Compaction requires no pending creation and
no other open file handles; it never forces an unmount. Failures are reported.

## Service and recovery

`com.sjaconette.dev-workspaces` runs at login and every 30 minutes via launchd.
Its registry is `~/.local/state/dev-workspaces/state.json`. Atomic writes and a
file lock serialize updates. Each enrolled worktree has a random identity token
in its Git administrative directory, so path reuse cannot authorize deletion.

Cleanup requires verified volume identity, explicit completion, retention expiry,
an unchanged branch/HEAD, preserved commits, a clean index and working tree,
no undeclared ignored data, no Git lock, no pin, no live lease, and no other open
file handles. Worktrunk removes in the foreground without force or branch deletion.
Its pre-remove hook repeats the checks. These checks rely on cooperating launchers;
they cannot eliminate races with arbitrary external writers bypassing the protocol.

Interrupted creations expire their pending reservation after 10 minutes. An
unregistered directory is never deleted. Crashed leases are reconciled by process
identity. Interrupted completed removal releases its registry entry on the next
sweep. Missing unfinished or identity-conflicting work stays reported for review.
No generic Git prune, force removal, or branch deletion is performed.

```sh
dev-workspace status
dev-workspace gc                 # preview eligible removals
dev-workspace gc --apply
dev-workspace mount
dev-workspace compact            # requires an empty managed-workspace registry
launchctl print gui/501/com.sjaconette.dev-workspaces
```

Status is saved to `~/.local/state/dev-workspaces/status.json`; errors go to
`service.log` with one rotated copy. The legacy inventory is
`legacy-inventory-20260912.json` in the same state directory. All 94 initial
linked registrations are excluded from automatic adoption or deletion.

SDD Harness retains its current workspace backend during its explicitly frozen
native-runtime migration. Follow-up `ynab-mcp-new-d44` tracks integration at the
native backend boundary. Its existing candidates and evidence remain untouched.

To stop scheduled cleanup:

```sh
launchctl bootout gui/501 ~/Library/LaunchAgents/com.sjaconette.dev-workspaces.plist
```

Then remove the four `storage` hooks from Worktrunk config if reverting the
lifecycle integration. Preserve the registry and image until all work is accounted
for. Worktrunk has native plugin uninstall commands; Gemini uses
`gemini extensions uninstall worktrunk`. Removing integration never deletes work.

## Development

```sh
cd ~/.local/share/dev-workspaces
./check.sh
go build -o dev-workspace .
```

The dotfiles pre-commit hook runs formatting validation, tests, vet, and plist
validation for storage changes. Tests use disposable Git repositories and verify
actual cleanliness, preservation, pins, process liveness, identity, retention,
capacity reservations, offline mounts, and interrupted lifecycle recovery.
