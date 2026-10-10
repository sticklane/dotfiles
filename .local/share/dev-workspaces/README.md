# Managed development workspaces

Native jj workspaces and Worktrunk Git worktrees share one managed lifecycle. `dev-workspace` supplies admission, process leases,
preservation checks, per-workspace scratch, shared native caches, and crash recovery.
The code uses only the Go standard library. Configuration and source are tracked in
`~/dotfiles-jj` (Git backend `~/.dotfiles.git`). Worktrunk itself is installed through Homebrew.

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

Before creating a task, inspect `dev-workspace status` and reuse an existing task.
To return to a native jj task, cd to its registered path; for Git use `wt switch`.
Then run `dev-workspace run codex` (or Claude/Gemini). Also use `dev-workspace run <command> [arguments]` for builds,
test runs, and temporary commands. It configures shared uv, npm, pip, Go/module,
Playwright, and Puppeteer caches on APFS, plus per-worktree `.dev-build` scratch
and Cargo build output. These environment values propagate to child processes.
Use Cabal's `--builddir=.dev-build/cabal` for Haskell builds.

When the task is done, preserve commits and required artifacts and run
`dev-workspace finish <path>`. An owning agent can mark completion before exiting;
its live lease continues to block removal until it exits. The worktree becomes
eligible after 24 hours. Native jj creation explicitly uses `--no-colocate` and starts unpinned but unfinished. Older
jj workspaces remain pinned until explicitly reviewed and unpinned. For authorized
immediate jj cleanup, preview `dev-workspace remove-jj <path>` then add `--apply`;
this skips only retention. `wt remove <path>` handles guarded immediate Git removal.
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

The new `/Volumes/dev-workspaces` APFS pool is a **128 GiB maximum sparse image** at
`/Volumes/SSK SSD/development/dev-workspaces.sparsebundle`. Its actual available
capacity is limited by the backing drive. The separate 24 GiB Fooszone image is
unchanged and is never unmounted or cleaned by this service.

The Seagate drive has ample capacity but is currently read-only NTFS. It was not
reformatted or remounted. SSK is therefore a guarded transitional destination.

`~/.config/dev-workspaces/config.json` records both volume UUIDs and the exact
mount points. A missing, wrong, suffixed, or read-only mount fails closed; there
is no internal-disk fallback. Worktrunk path settings live in
`~/.config/worktrunk/config.toml` and include repository and branch identity.

Each pending creation and workspace with a live process lease reserves 4 GiB
of future growth. Idle retained checkouts reserve no additional growth; their
allocated bytes are already reflected in free space. Acquiring a lease rechecks
capacity under the same registry lock, and nested leases count once per workspace.
Admission serializes reservations, includes in-flight creations, and requires 8 GiB free in the pool, 12 GiB on SSK, and 2 GiB
internally **after** reservations where relevant. Maximum present-workspace count is 64, including pending creations;
missing records remain visible but do not consume slots. Disk budgets still apply. These are admission checks,
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
file handles. Open-file checks use a bounded kernel snapshot with NUL-delimited
fields, including cwd and nested descriptors, instead of recursively scanning caches.
Native immediate removal must run outside the target; even its own caller/ancestors
count as active owners. Completion may still be recorded by the owning agent. Worktrunk removes in the foreground without force or branch deletion.
Its pre-remove hook repeats the checks. These checks rely on cooperating launchers;
they cannot eliminate races with arbitrary external writers bypassing the protocol.

Interrupted Git admission reservations expire after 10 minutes. Native jj creation
records never expire: `jj_creations` retains the preselected identity, exact base,
repository, name, path, and start time. They reserve capacity and remain excluded
from leasing, completion, and collection. An unregistered directory is never deleted. Crashed leases are reconciled by process
identity. Interrupted completed removal releases its Git registry entry on the next
sweep. JJ interruptions remain explicit recovery records. Missing unfinished or identity-conflicting work stays reported for review.
No generic Git prune, force removal, or branch deletion is performed.

Native jj cleanup snapshots tracked files without auto-tracking new files, then
requires the exact revision on a remote bookmark or an undescribed empty change
with one remotely preserved parent. Local main alone is insufficient. It inventories
untracked and ignored files (including unknown empty directories) without following
symlinks. Only the documented disposable directories are exempt. A stale workspace
fails closed; cleanup never updates it automatically. The random token, workspace
name/revision, external shared repository, leases, and open files are rechecked.

The snapshotting `jj status` used by finish and removal, and the later
`jj workspace forget`, have no process-killing command deadline. Even a preview
can snapshot jj state. Read-only metadata commands keep the bounded query
deadline; jj metadata reads use `--ignore-working-copy`. That flag does not make
an explicit mutation such as `workspace forget` read-only. Observe slow operations
without terminating them, and retain the owning process/session until completion.

Under one continuous registry lock, the collector persists intent and moves the workspace to a token-bound sibling
quarantine path, rechecks its contents and owners, forgets only its jj registration,
then removes the quarantined directory. It never abandons commits, prunes history, or touches sibling
workspaces. Interrupted removal retains a `removing` registry record with `removal_path` (also derivable as a sibling `.removing-<identity-token>` if an
older in-memory manager drops the optional field); automatic retries are disabled, including when the original path is missing. Review the exact record, identity,
filesystem contents, shared repository and preserved revision for manual recovery.
Do not re-adopt or force-delete it. Even if deletion completed but registry save failed, jj reconciliation retains
the missing entry for manual review; it never assumes forget succeeded.


Removal apply can take time on large disposable caches. Do not impose a wrapper
timeout that kills the remover: use an asynchronous session and an observation
deadline that reports progress without terminating it. A tool yield is not a process
timeout. Preserve the session/process identity until completion.

### Interrupted native jj creation

Creation captures the exact parent revision before checkout and persists intent
before running `jj workspace add`. That mutation has no internal wall-clock timeout;
watch it asynchronously and do not kill it merely because external storage is slow.
A failed command retains its intent and any partial directory. Do not retry under a
new task name to bypass the reservation. Status exposes `registry.jj_creations`.

Recovery is deliberately manual; `recover-jj` handles removal, not creation. Save the
record, confirm the creator and its children have exited, and inspect the exact path,
shared repository, workspace registration/revision, token, and complete file inventory.
A recorded base and registered name alone do not prove checkout completed. Neither
path nor registration permits reviewed cancellation of only the intent. Both present
require an exact empty child of the recorded base and complete matching contents
before reviewed finalization with the preselected identity. Path-only, registration-only,
foreign token, wrong revision/repository, or unknown contents must remain retained.
Do not delete, forget, update shared history, adopt an unrecorded directory, or clear
an intent to bypass an inconsistency. Any approved record repair holds `state.lock`
and preserves before/after evidence. There is no automatic recovery command yet.

Registry v2 protects these records from older binaries that would otherwise drop
unknown fields. The first locked write migrates v1 without changing existing leases
or workspace records. Install the tested binary before migration. Older running
launchers may fail their final registry updates; retain their leases until the new
manager verifies their process has exited. Never downgrade the registry version.

### Interrupted native jj removal

Automatic retries remain disabled. Save the exact record and inspect both `path`
and the token-derived sibling quarantine path (`removal_path` is advisory), their
identity tokens, the shared repository, registration/revision, preservation, and
active owners. A missing optional field may come from an older loaded manager;
the existing token and `removing` flag still identify the recovery state.

From outside the target workspace, preview `dev-workspace recover-jj <original-path>`.
It requires exactly one original/quarantine tree, an intact registered jj workspace,
unchanged preserved revision, and all ordinary safety checks. Add `--apply` to cancel
intent (or rename a verified quarantine back), pin the workspace, and reset completion.
This command does not delete files or re-adopt forgotten work. Review it again before
unpinning, finishing, and trying removal. Active owners block quarantine renames.

If the registration was forgotten, both trees are absent, or identity/preservation
checks fail, keep the remaining files and registry for narrowly reviewed manual
recovery. Do not automatically recreate the name or restore shared operation history.
A record may be cleared manually under `state.lock` only after exact matching identity,
absence of both paths, and completed forget are independently established.

Registry repair must not waive a failed data-preservation or ownership check. Never
remove shared `.jj/repo`, abandon commits, or restore repository-wide operation
history to repair one workspace. Keep the saved before/after record as evidence.

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
# In a managed workspace of ~/dotfiles-jj:
dev-workspace run sh .local/share/dev-workspaces/check.sh
dev-workspace run sh -c 'cd .local/share/dev-workspaces && go build -o ../../../.dev-build/dev-workspace .'
# Explicitly install reviewed sources/config and the tested binary into home.
```

The dotfiles pre-commit hook runs formatting validation, tests, vet, and plist
validation for storage changes. jj does not run Git hooks; run check.sh explicitly. It resolves its own source tree,
not the installed home copy. Tests use disposable Git and jj repositories and verify
actual cleanliness, preservation, pins, process liveness, identity, retention,
capacity reservations, offline mounts, and interrupted lifecycle recovery.

For a bounded diagnostic when external APFS I/O stalls, the managed runner may use
an existing shared internal Go cache and a small disposable internal test directory.
Override **both** `TMPDIR` and `GOTMPDIR`; verify their effective values before tests.
Keep source checkout/leases managed, record the exception, and do not create a new
per-run cache. The normal build-cache policy remains unchanged.
