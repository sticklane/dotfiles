---
name: managed-workspaces
description: Create, resume, finish, inspect, or clean managed development workspaces on this Mac, or repair dev-workspace janitor policy. Use for workspace lifecycle tasks and when starting or completing work in a managed workspace. Applies to native jj and Git/Worktrunk workspaces; excludes automatic adoption or deletion of Codex-owned and legacy worktrees.
---

Read `~/.local/share/dev-workspaces/README.md` for the installed lifecycle contract.
For VCS changes, read `~/.config/jj/WORKFLOW.md`. Use canonical `~/dotfiles-jj` for
janitor code/config and this skill; explicitly install tested changes into home.
Do not edit plugin caches as durable policy.

## Start and resume

Inspect `dev-workspace status` and repository/workspace identity. Reuse a registered
path for the same task. Otherwise use `dev-workspace create <task>` in jj repositories
or `dev-workspace start codex|claude|gemini <task>`. Review the intended base: native
jj creation uses `@-`. Run builds/scratch through `dev-workspace run` for APFS caches
and a process lease. Never change another task's history or manufacture `.git` links.
If admission is full, inspect completed candidates; do not increase the persistent
cap or delete idle work merely to make room. Any maintenance exception must be
bounded, explicit, and leave the normal capacity and preservation gates in place.

A slow jj checkout has no internal mutation deadline. Observe it without killing
it. If creation fails, inspect `registry.jj_creations` and follow the README's
interrupted-creation procedure; do not create a replacement task, use removal
recovery, or erase its reservation. The record retains capacity until explicit
review. Registry v2 rejects older writers; after installing an update, keep any
old launcher's remaining lease until process identity proves it has exited.

## Complete

Before ending a completed managed task, review the diff and all untracked/ignored
data, run required checks, and preserve every authored change remotely using the
named-bookmark and dry-run workflow. Preserve needed artifacts outside disposable
scratch. Run `dev-workspace finish <path>` and verify that completion is recorded.
Finish starts 24-hour retention. An owner may finish before its launcher exits;
its lease still blocks collection. If work is intentionally retained, pin it and
report why. If finish fails, report the concrete blocker; do not silently abandon
another unfinished workspace or override the guard.

Native jj workspaces now start unpinned but unfinished. Existing native workspaces
remain pinned. Unpin older workspaces only after a specific ownership/content review;
then finish. For jj, an unpublished anonymous change is not preserved just because
local main exists. An empty working change is eligible only if undescribed and its
single parent is remotely preserved. Preserve a described empty change by publishing
it; do not erase meaningful task notes merely to satisfy the empty-child exception. Every untracked/ignored non-disposable file
and even an unknown empty directory blocks deletion.

## Clean up

Keep inventory bounded: do not launch recursive `du`/`find` across the pool or
rebuildable caches to decide ownership. Use registered identity, preserved revisions,
and the janitor's bounded kernel open-file check. Account for and stop your own
unfinished inventory/test processes before handing off; a tool timeout does not
prove its process exited. Never terminate another task's process to free capacity.

A user's cleanup request authorizes review and removal of eligible specified work;
no repeated permission is needed once scope is clear. Preview `dev-workspace gc`
then apply with `gc --apply` for finished work past retention. For explicitly
requested immediate native jj cleanup, finish it, preview
`dev-workspace remove-jj <path>`, inspect the exact path/revision, then apply with
`--apply` from outside the target workspace. This skips only retention; the
caller and its ancestors also count as active owners during removal. Never call `rm -rf`, `jj workspace forget`,
force removal, or registry edits to bypass a failed preservation/identity check.
Do not wrap removal apply in a process-killing wall-clock timeout. Large disposable
caches can take time to delete; use an asynchronous session and a watcher whose
deadline reports status without killing the remover. If interrupted, stop and use
the recovery procedure before any retry.
Do not infer ownership release from age, missing activity markers, task closure,
or an empty current change. Active leases/open files, dirty or unpublished work,
unknown ignored data, identity conflicts, and stale jj workspaces fail closed.
Do not sweep Codex-managed, legacy, unregistered, or externally owned directories.

After cleanup, verify registry count and remaining blockers. Any interrupted jj removal leaves a recovery record with a `removal_path`; stop
and inspect that exact record and quarantine directory (even if the original path
is missing) rather than repeatedly applying cleanup. Follow the README interruption decision tree. Preview `dev-workspace recover-jj
<original-path>` and apply only that verified recovery: it restores a still-registered
preserved tree, pins it, and resets completion. It never re-adopts forgotten work.
If an older loaded manager dropped `removal_path`, derive the sibling
`.removing-<identity-token>` from the record; never substitute an arbitrary path. A registry repair may
restore a verified interrupted state, never waive preservation/ownership checks.
Shared jj commits, operation history, sibling workspaces, and remote bookmarks
must remain intact.

## Repair the janitor

Inspect canonical versus installed code/config before editing. Use a managed
workspace, run its own `.local/share/dev-workspaces/check.sh` through the managed
runner, and independently review deletion-path changes. Tests must cover unpublished,
ignored, active, replaced, and interrupted work as well as successful cleanup.
Install only reviewed sources/config and the tested binary; preserve unrelated home
instructions. Verify launchd uses that installation and preview against real state
before removal. For slow external I/O during bounded tests, verify both `TMPDIR`
and `GOTMPDIR` when using a documented temporary override and an existing shared cache.
Update README, jj workflow, and relevant agent lifecycle rules with
behavior changes. Report reclaimed workspaces, protected work, and residual risks.
