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
