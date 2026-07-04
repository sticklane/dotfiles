# Unified Development Guidelines

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
4.  **Is there Tech Debt?** If you spot technical debt or future improvements, add it to the repo's `docs/TASKS.md` (or open a spec under `specs/` for larger work).

## Code Quality
- **Type Safety**: We strongly prefer statically typed languages (e.g., TypeScript, Go, Rust). For existing JavaScript codebases, prioritize converting to TypeScript. Use JSDoc only as a temporary measure.
- **Linting**: Ensure code passes all linting rules.
- **Testing**: Maintain high test coverage.
- **Debugging**: When debugging UI issues or verifying changes, prefer using the browser tool to take screenshots (`screenshot`) to visually verify the state of the application. Iterate quickly by checking the visual output.

## Change Process
- **Commit frequently**: Commit changes often to keep the history clean and easy to follow.
- **Commit messages**: Use clear and concise commit messages that describe the changes made.
- **Commit hooks**: Gated repos use a two-layer system — the git pre-commit hook runs fast staged-file checks only (format + lint; `--no-verify` to bypass), and the Claude Code Stop hook runs the repo's full `scripts/check.sh` (lint, typecheck, tests) before work can be called done.
- **Be aware of staged changes**: It is good practice to check for and commit staged changes before starting a new task.
- **Commit conflicts**: Resolve conflicts before committing.
- **Commit history**: Keep the commit history clean and easy to follow.
- **Push after commit**: Push changes to the remote repository after committing.
- **Check for pending uncommitted changes**: Check for pending uncommitted changes before committing.
- **Don't be afraid to split commits into multiple smaller commits**: Don't be afraid to split commits into multiple smaller commits to make the history easier to follow and keep changes focused and easy to revert.
- **After comitting, take a moment to reflect: what could be improved?**: After comitting, take a moment to reflect: what could be improved? Are there any issues that could be closed? Are there any tasks that could be created to track the next steps?
- **Commit on Task Completion**: Always commit changes when a task is completed. Do not leave changes uncommitted when moving to the next task.

## Workflow Preferences
- **Chained Execution**: The user prefers to chain multiple tasks together without pausing for implementation plan reviews.
- **Rapid Development**: Skip plan reviews unless there is high ambiguity or risk. Proceed directly to execution after creating the plan.
- **Record plans**: When you create a plan, always record it in the task file (or `docs/TASKS.md`).

## Docker / Container Builds
- **ALWAYS use Cloud Build** for Docker images. NEVER build Docker images locally — local disk is too limited and Docker Desktop is unreliable.
- Build command: `gcloud builds submit --tag <image-uri> <context-dir> --timeout=1200`
- Ensure `.gcloudignore` excludes large data dirs (test_frames, .venv, checkpoints, etc.)
- The project uses Artifact Registry at `us-central1-docker.pkg.dev/fooszone/fooszone/`

## Visual Test Log
When visual regression tests need snapshot updates due to intentional UI changes, document the change in `docs/visual-test-log.md`. This log serves to:
- Track patterns in which tests break most often
- Identify opportunities to improve test targeting/isolation
- Build evidence for decisions about test strategy adjustments
- Help assess if tests are providing value vs. creating friction

Each entry should note: what changed, which tests were affected, and observations about whether the test behavior was helpful.
