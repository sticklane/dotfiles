# Unified Development Guidelines

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

## Core Philosophy
- **TDD (Test-Driven Development)**: Write tests before writing code. Ensure tests fail before making them pass.
- **Modularity**: Code should be broken down into small, single-purpose functions and components.
- **Simplicity**: Avoid over-engineering. Choose the simplest solution that works and is maintainable.
- **Flexibility**: Design for change, but don't anticipate every possible future requirement.
- **External Dependencies**: Treat new dependencies with extreme caution. Always prefer using existing tools and libraries. If a new dependency is absolutely necessary, justify why existing solutions are insufficient.
- **Battle-Tested Technologies**: Prioritize established, widely-used technologies over new or experimental ones. Reliability and community support are paramount.

## Agent Review Process
Whenever code is written, the agent must take an extra pass to answer:
1.  **Does it make sense?** Is the logic sound and easy to follow?
2.  **Does it follow best practices?** Are we using modern patterns and avoiding anti-patterns?
3.  **Could it be simpler?** Can we reduce complexity without sacrificing functionality?
4.  **Is there Tech Debt?** If you spot technical debt or future improvements, create a task in Beads (`bd create "Title" -t chore`).

## Code Quality
- **Type Safety**: We strongly prefer statically typed languages (e.g., TypeScript, Go, Rust). For existing JavaScript codebases, prioritize converting to TypeScript. Use JSDoc only as a temporary measure.
- **Linting**: Ensure code passes all linting rules.
- **Testing**: Maintain high test coverage.
- **Debugging**: When debugging UI issues or verifying changes, prefer using the browser tool to take screenshots (`screenshot`) to visually verify the state of the application. Iterate quickly by checking the visual output.

## Change Process

Follow the Jujutsu workflow above. Keep changes focused, validate them, and publish
only under the repository's active publication policy. Preserve unrelated work.

## Workflow Preferences
- **Chained Execution**: The user prefers to chain multiple tasks together without pausing for implementation plan reviews.
- **Rapid Development**: Skip plan reviews unless there is high ambiguity or risk. Proceed directly to execution after creating the plan.
- **Record plans**: When you create a plan, always record it in the bead.

## Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

### Quick Start

**Check for ready work:**
```bash
bd ready --json
```

**Create new issues:**
```bash
bd create "Issue title" -t bug|feature|task -p 0-4 --json
bd create "Issue title" -p 1 --deps discovered-from:bd-123 --json
bd create "Subtask" --parent <epic-id> --json  # Hierarchical subtask (gets ID like epic-id.1)
```

**Claim and update:**
```bash
bd update bd-42 --status in_progress --json
bd update bd-42 --priority 1 --json
```

**Complete work:**
```bash
bd close bd-42 --reason "Completed" --json
```

## Managing AI-Generated Planning Documents

AI assistants often create planning and design documents during development:
- PLAN.md, IMPLEMENTATION.md, ARCHITECTURE.md
- DESIGN.md, CODEBASE_SUMMARY.md, INTEGRATION_PLAN.md
- TESTING_GUIDE.md, TECHNICAL_DESIGN.md, and similar files

**Best Practice: Use a dedicated directory for these ephemeral files**

**Recommended approach:**
- Create a `history/` directory in the project root
- Store ALL AI-generated planning/design docs in `history/`
- Keep the repository root clean and focused on permanent project files
- Only access `history/` when explicitly asked to review past planning

## Agent Coordination (Branch Workflow)
This project uses a branch-based workflow for agent coordination.

1. **Worker Agents**:
   - Pick a task from `bd ready`.
   - Create/Checkout a branch `task/<task-id>`.
   - Implement changes.
   - Commit changes.
   - Reassign task to `merger` agent.
   - Add comment: "Work completed on branch `task/<task-id>`. Ready for merge."

2. **Merger Agent**:
   - Watches for tasks assigned to `merger`.
   - Merges `task/<task-id>` into `main`.
   - Resolves conflicts using LLM if needed.
   - Closes the task upon success.

## Using bv as an AI sidecar

bv is a fast terminal UI for Beads projects (.beads/beads.jsonl). It renders lists/details and precomputes dependency metrics (PageRank, critical path, cycles, etc.) so you instantly see blockers and execution order. For agents, it’s a graph sidecar: instead of parsing JSONL or risking hallucinated traversal, call the robot flags to get deterministic, dependency-aware outputs.

*IMPORTANT: As an agent, you must ONLY use bv with the robot flags, otherwise you'll get stuck in the interactive TUI that's intended for human usage only!*

- bv --robot-help — shows all AI-facing commands.
- bv --robot-insights — JSON graph metrics (PageRank, betweenness, HITS, critical path, cycles) with top-N summaries for quick triage.
- bv --robot-plan — JSON execution plan: parallel tracks, items per track, and unblocks lists showing what each item frees up.
- bv --robot-priority — JSON priority recommendations with reasoning and confidence.
- bv --robot-recipes — list recipes (default, actionable, blocked, etc.); apply via bv --recipe <name> to pre-filter/sort before other flags.
- bv --robot-diff --diff-since <commit|date> — JSON diff of issue changes, new/closed items, and cycles introduced/resolved.

Use these commands instead of hand-rolling graph logic; bv already computes the hard parts so agents can act safely and quickly.