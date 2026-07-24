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
- Gated repos have a two-layer gate: the VCS's pre-commit hook runs fast
  staged-file checks only (format + lint; bypass with the VCS-appropriate
  flag, e.g. git's `--no-verify`), and
  the Stop hook runs the full `scripts/check.sh`. Don't hand-write
  formatting/lint reminders into workflows — the hooks own that.
- Repos without gates: run the repo's own lint/typecheck/test commands
  before finishing.

## CI cost discipline (metered runners)

Push-per-commit conventions multiply CI: every commit an agent pushes is a
billed run on private repos (learned the hard way 2026-07 — the account's
Actions budget was exhausted and all runners blocked account-wide). Every
push-triggered pipeline on a metered runner (GitHub Actions, Cloudflare
Workers Builds, Cloud Build triggers, …) MUST ship with all of:

- `concurrency` group per ref + `cancel-in-progress: true` — burst pushes
  supersede each other; never let a stale run finish (~2/3 of runs here
  were superseded within 10 min). Exception: deploys may queue instead.
- `paths-ignore` for docs-only pushes: `**.md`, `docs/**`, `specs/**`,
  `.claude/**` — baton-pass/spec commits are the bulk of push traffic.
- `timeout-minutes` on every job (default is 6 h).
- Multi-stack repo: one workflow per stack with path filters, never one
  workflow running every stack on every push. Deploy/build watch paths
  (e.g. Workers Builds `path_includes`) get the same scoping.
- `on: push` must name branches — a bare `on: push` + `pull_request`
  double-runs every PR branch.

Never make a path-filtered job a _required_ status check without a no-op
success fallback — skipped runs leave PRs stuck "Expected".

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
- Toolkit skills are plugin-namespaced: invoke via the Skill tool as
  `agentic:<name>` (e.g. `agentic:distill`) even when absent from the
  session's skill listing — a bare `<name>` lookup wrongly reports it
  unavailable (bit drain's terminal distill self-chain, 2026-07-20).
- `~/.claude/skills/` holds personal skills only. Personal skills are not
  tracked in dotfiles: each lives in its own home repo and is symlinked in,
  or (for private ones) lives here untracked with a self-excluding
  `.gitignore` so no repo ever picks it up. The toolkit dev checkout is
  `~/claude`.

## Repo navigability

- Active repos use the orientation split: root `AGENTS.md` = orientation
  (purpose, `## Map`, `## Commands` — verified by running, `## State`);
  `CLAUDE.md` = conventions + an `@AGENTS.md` bridge line near the top; both
  ≤200 lines; `README.md` for humans. Never write a command you didn't just
  run.
- In a repo missing these, offer /onboard rather than ad-hoc fixes.
- `~/REPOS.md` (regenerated daily by `com.sjaconette.repo-index`) audits
  compliance; a ✗ on a std-marked row is drift to fix.

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

## Writing quality (human-facing output only — never reasoning)

The prose-review doctrine (agentic plugin; full rubric with sources in
`~/claude/.claude/skills/prose-review/reference.md`) applies to everything
written FOR A HUMAN READER — responses, summaries, commit messages, reports,
PR/issue text — not just docs under review. It does NOT apply to thinking /
chain-of-thought: reasoning is model-consumed working text, and constraining
its style is uncompensated risk (same evidence as the machine-parsed rule
below). Think however works best; apply the rubric only when composing what
the human reads. The always-on core:

- **No AI antipatterns** (the nine-item rubric): bullets fragmenting what
  reads better as prose (genuinely discrete/enumerable lists are exempt);
  hedging pileups and AI-reminders; sycophancy; over-formatting (bold,
  headers, lists beyond what aids scanning); purple prose and clichés; stock
  openers ("Got it", "Great question"); repetition/length without added
  content; blurry words ("things", "stuff") where a concrete specific
  belongs; progress narrated as achievement instead of reported as fact.
- **No agentic-register tells**: meta-discourse ("Let me check…", "Now
  I'll…" — do the thing, then state the result); false precision ("~40
  lines", "well under a minute" on a number already in hand — state it
  exactly); evaluative varnish ("clean", "solid", "robust" standing in for
  the checkable fact — "214 tests pass, 0 fail" beats "the suite is solid").
  Carve-out: terse status lines that state verifiable facts ("gates green",
  "merged and pushed") are exempt — never pad them into prose.
- **Google-style structure**: audience-first ordering (the most important
  thing at the top); one idea per paragraph; conditions before instructions;
  concrete over abstract (show the command, the path, the value);
  descriptive link text (never "click here"); define acronyms/jargon on
  first use.

Before writing or substantially reshaping a human-facing doc (README.md,
AGENTS.md, docs/*.md), load `agentic:prose-review`'s authoring doctrine
first (Diátaxis quadrant selection + Google essentials); review nontrivial
doc work with `/prose-review` before calling it done.

**Machine-parsed prose (task files, specs, SKILL.md bodies, prompts) is a
different register — research does not support human-style polish there.**
What measurably helps a model consumer: conciseness (padding the same task
from ~250 to ~3,000 tokens drops accuracy ~0.92→0.68 — Levy et al., ACL
2024), typo-free exact wording (perturbations cost 17–33% accuracy —
PromptRobust), and key content front-loaded or last, never mid-document
("Lost in the Middle"). What to avoid: style-polish rewrite passes
("best-practice" prompt rewrites measurably degraded task compliance —
arXiv:2601.22025) and gratuitous reformatting of working docs (format
effects are large, model-specific, and directionless — up to 76 points from
separators/casing alone, FormatSpread; the one head-to-head test found
neither bullets nor prose reliably better, arXiv:2607.19257). So: keep a
working doc's existing format; edit for brevity, precision, and placement —
never for style. The rubric's conciseness (item 7) and concreteness (item 8)
transfer to this register; its prose-over-bullets preference does not.

## Private-skill hygiene (binding, all projects)

One personal skill is private and local-only, referenced everywhere by its
codename **cedar-mill-review**. Nothing about it — subject matter, real
names, trigger words, paths, slugs, findings — may EVER be written to any
git repo (tracked files, commit messages, specs, task lists) or any deployed
/cloud destination. Its tasks, findings, and evidence live only in its own
local directory; read `~/.claude/skills/cedar-mill-review/SKILL.md`'s
PRIVACY section before touching it. Repo references say at most "a private
local-only skill (cedar-mill-review)". Verify across all repos (trees AND
histories) with `~/cedar-mill-review/leak_scan.sh` — zero hits is the pass;
repair violations by scrubbing AND rewriting affected history, never by a
follow-up commit.
