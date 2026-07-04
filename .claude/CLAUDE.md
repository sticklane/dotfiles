# Claude Development Guidelines

## TDD — red, green, refactor

1. **RED** — write a failing test first; run it and confirm it fails for the
   right reason. A test that passes immediately isn't testing anything new.
2. **GREEN** — write the minimal code that makes it pass. No extra features
   "just in case".
3. **REFACTOR** — improve structure with the tests green; never change
   behavior and tests in the same step.

Always write tests for: new features (test first), bug fixes (failing repro
first), refactors (safety net), public APIs, business logic, edge cases.
Optional for: logic-free getters/setters, pure UI (prefer integration tests),
thin third-party wrappers (test your usage, not the library).

Test rules of thumb:
- Test behavior, not implementation — assert what it does, never internals
  or exact output strings (parse, then assert structure).
- One behavior per test; fresh objects per test; any execution order.
- Mock only slow/external dependencies — over-mocking tests nothing real.
- Every test asserts something; "runs without error" is not a test.
- Names describe scenario and expectation:
  `test_detector_returns_none_when_no_ball_in_frame`.

## Checks and gates

- `scripts/check.sh` is the canonical per-repo check (lint, typecheck,
  tests). Run it green before calling work done — in gated repos the Stop
  hook enforces exactly this.
- Gated repos have a two-layer gate: the git pre-commit hook runs fast
  staged-file checks only (format + lint; bypass with `--no-verify`), and
  the Stop hook runs the full `scripts/check.sh`. Don't hand-write
  formatting/lint reminders into workflows — the hooks own that.
- Repos without gates: run the repo's own lint/typecheck/test commands
  before finishing.

## Commits

- Small, focused, atomic commits; commit at each TDD step (test → feat →
  refactor) and when a task completes. Never leave finished work
  uncommitted.
- Format: `<type>: <subject>` — types: feat, fix, test, refactor, docs,
  style, perf, chore.
- Don't commit: debugging prints, commented-out code, broken tests, mixed
  unrelated changes (split them).
- Push after committing only when the task/repo conventions say so; some
  repos auto-push on commit (their hooks handle it).

## Skills and agents

- Toolkit skills (/build, /drain, /breakdown, scout/critic/verifier agents,
  …) are served by the `agentic` plugin — never copy or symlink them into
  `~/.claude/skills/`; copies shadow the plugin and go stale.
- `~/.claude/skills/` holds personal skills only. Personal skills are not
  tracked in dotfiles: each lives in its own home repo and is symlinked in,
  or (for private ones) lives here untracked with a self-excluding
  `.gitignore` so no repo ever picks it up. The toolkit dev checkout is
  `~/claude`.

## Token discipline

Context is the scarce resource; pollution compounds turn over turn. Spend
main-context tokens on decisions; delegate raw-material consumption.

- Never read files into main context to "look around" — use the `scout`
  agent for where/how/what-exists questions; fan scouts out in parallel.
  Read a file directly only when about to edit it, and prefer the relevant
  slice.
- Verification, review, and research belong in subagents (`verifier`,
  `critic`, `Explore`) — their transcripts are discarded; only the final
  report costs context.
- Match model to work: mechanical/lookup → cheap tier (scout default);
  judgment → session model. Don't pay frontier rates to run grep.
- One task per session; `/clear` between tasks. Resumable state lives in
  artifacts on disk (specs, task files, handoffs), never in conversation
  memory. Summarize command output instead of pasting it.
- Cheap before expensive: critique the spec before implementing it; make
  acceptance checks runnable commands so verification is one cheap
  subagent.
