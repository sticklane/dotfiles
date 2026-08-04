# Claude Development Guidelines

## Work tracking — bd (beads) is the tracker

Any repo with a `.beads/` directory tracks work in bd. As of 2026-08-03:
automation, budget_analysis, fooszone, hub, interview-prep,
portfolio-tracker, specs, ynab-mcp-new. **There is no `/work` skill** — it
was removed with the agentic-toolkit plugin on 2026-07-31 and is not coming
back. This section IS the procedure; follow it directly, in every session,
without being asked.

- **Session start:** the SessionStart hook already runs `bd prime` — read its
  output instead of re-running it. `bd ready` is the dispatchable queue.
- **Claim before working:** `bd update <id> --claim`, then append that id to
  `.beads/session-claims` (one per line). The Stop hook reads that file.
- **Close on done:** `bd close <id>` and delete its line from
  `.beads/session-claims`. Never end a session holding a claim — unclaim it
  if the work didn't land.
- **File discovered work the moment you spot it:**
  `bd create "<title>" --deps discovered-from:<current-id>`. Tech debt,
  follow-ups, and drive-by bugs go here, never into a markdown file.
- **Do NOT use TodoWrite or TaskCreate/TaskUpdate to hold work state in a bd
  repo.** The harness emits recurring "consider using TaskCreate" reminders;
  in a bd repo that nudge is wrong. Built-in task tools are scratch
  sequencing within one turn — never the record.
- `docs/TASKS.md` in these repos is **inert historical record**. Do not add
  to it, do not tick its boxes.
- Repos without `.beads/`: fall back to `docs/TASKS.md` or a spec under
  `specs/`.

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

## Skills

- `~/.claude/skills/` holds personal skills only. Personal skills are not
  tracked in dotfiles: each lives in its own home repo and is symlinked in,
  or (for private ones) lives here untracked with a self-excluding
  `.gitignore` so no repo ever picks it up.

## Repo navigability

- Active repos use the orientation split: root `AGENTS.md` = orientation
  (purpose, `## Map`, `## Commands` — verified by running, `## State`);
  `CLAUDE.md` = conventions + an `@AGENTS.md` bridge line near the top; both
  ≤200 lines; `README.md` for humans. Never write a command you didn't just
  run.
- In a repo missing these, offer to write the set as a unit rather than
  patching one file ad hoc.
- `~/REPOS.md` (regenerated daily by `com.sjaconette.repo-index`) audits
  compliance; a ✗ on a std-marked row is drift to fix.

## Token discipline

Context is the scarce resource; pollution compounds turn over turn. Spend
main-context tokens on decisions; delegate raw-material consumption.

- Never read files into main context to "look around" — delegate
  where/how/what-exists questions to a read-only search subagent, and fan
  several out in parallel for independent questions. Read a file directly
  only when about to edit it, and prefer the relevant slice.
- Verification, review, and research belong in subagents — their
  transcripts are discarded, so only the final report costs context.
- Match model to work: mechanical/lookup → cheap tier; judgment → session
  model. Don't pay frontier rates to run grep. When dispatching subagents,
  load the `superpowers:subagent-driven-development` skill and follow its
  `## Model Selection` section to pick each agent's model — use the least
  powerful model that can do the role: mechanical implementation (isolated
  functions, clear spec, 1–2 files) → fast/cheap; integration, debugging,
  judgment → standard; architecture/design and late-round fix attempts →
  most capable available, NOT the session default. Set the model explicitly
  per agent; never let every subagent silently inherit the session model.
- One task per session; `/clear` between tasks. Resumable state lives in
  artifacts on disk (specs, task files, handoffs), never in conversation
  memory. Summarize command output instead of pasting it.
- Cheap before expensive: critique the spec before implementing it; make
  acceptance checks runnable commands so verification is one cheap
  subagent.

## Writing quality (human-facing output only — never reasoning)

This doctrine applies to everything written FOR A HUMAN READER — responses,
summaries, commit messages, reports, PR/issue text — not just docs under
review. It does NOT apply to thinking /
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
- **No agent-voice tells**: meta-discourse ("Let me check…", "Now
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

**Invoke the `prose-review` skill — do not just recall this rubric from
memory.** Load it BEFORE drafting or substantially reshaping any
human-facing doc (README.md, AGENTS.md, docs/*.md, specs/ prose): it carries
the Diátaxis quadrant choice plus the Google essentials. Run it AFTER any
nontrivial human-facing writing (doc, PR/issue body, report, summary) to
check the draft against the nine-item antipattern rubric and Vale before
calling the work done. Do this unprompted — Steven does not have to ask, and
`/prose-review` is the same skill. It does not cover code, docstrings, or
machine-parsed prose (see below).

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

## Superpowers and ultracode (binding, all projects)

Superpowers 6.2.0 is installed from `claude-plugins-official`. Its skills are
the methodology (TDD, systematic-debugging, brainstorming, writing-plans,
subagent-driven-development); these rules govern WHEN to spend the pipeline and
at what effort.

**Match the pipeline to the work.** The full brainstorm → plan →
subagent-driven-development run is for complex or ambiguous work. For a small
fix or a simple edit, work directly: on a trivial task the planning overhead
costs more than it saves. The skills themselves still apply — a one-line bugfix
still gets a failing test first — but the orchestration does not.

**Effort discipline.** Orchestrator, planning and review phases run at high or
xhigh; exploration runs LOWER, because breadth beats depth when you do not yet
know what you are looking for. Mechanical implementation subagents run at lower
effort and cheaper model tiers wherever the plan is concrete enough to carry
them — an isolated function against a clear spec does not need the frontier.
Set effort explicitly per subagent via skill/subagent frontmatter where
supported; never let every subagent silently inherit the session model.

**Never cap or suppress extended thinking on the orchestrator.** Thinking buys
turn efficiency: the tokens it spends are repaid by the tool calls, wrong
turns and re-reads it avoids. Capping it is a false economy.

**Reviewers get the brief, never the diff alone.** A reviewer handed only a
diff can check whether the code is self-consistent; it cannot check whether the
code does what was asked. Ship the task brief or spec alongside the review
packet, always.

**Durable state lives on disk.** Plans and progress go in markdown files, not
in conversation context — context is summarised away, files are not. This is
the same rule as the repo-level "resumable state lives in artifacts".

**Ultracode is never combined with a Superpowers run.** NEVER enable ultracode
while a subagent-driven-development run is active: two orchestration layers on
one task duplicate the decomposition and multiply the spend. Ultracode is for
large work OUTSIDE the pipeline — whole-codebase audits, mass parallel
refactors, deep exploration — and Steven invokes it manually, per session. It
is session-only by design; do not try to persist it in a settings file.

**Ultracode preflight.** It silently loses its workflow orchestration unless
`CLAUDE_CODE_EFFORT_LEVEL` is unset or `xhigh`, and it needs a model that
supports xhigh (Opus 4.8 or Opus 5). Check both before assuming an ultracode
turn is actually orchestrating.

**`CLAUDE_EFFORT` is not a config knob — do not "fix" it.** Both names are real
in the CLI binary, and they run in opposite directions. `CLAUDE_CODE_EFFORT_LEVEL`
is the INPUT override the preflight above checks. `CLAUDE_EFFORT` is an OUTPUT:
the binary documents it as "active effort level for the current turn … also
exposed to hook commands and Bash", so seeing `CLAUDE_EFFORT=high` in `env` is
Claude Code reporting effort to a subprocess, not a setting holding it there.
Verified 2026-08-04 against 2.1.220/2.1.221 after an audit misread it as a
misconfiguration.

**Three of the plugin's reviewer templates ship with no `model:` line, and
inherit silently.** The subagent-driven-development templates
(`implementer-prompt.md`, `task-reviewer-prompt.md`, `re-review-prompt.md`)
carry an explicit `model: [MODEL — REQUIRED …]` placeholder that forces the
choice. These three do NOT, and default to the session model — the most
expensive one — which is exactly what the effort-discipline rule above forbids:

- `skills/requesting-code-review/code-reviewer.md`
- `skills/brainstorming/spec-document-reviewer-prompt.md`
- `skills/writing-plans/plan-document-reviewer-prompt.md`

Set the model explicitly when dispatching any of the three. Do not patch the
plugin cache to fix it — an update overwrites the cache and the edit vanishes
without warning.

**Brainstorm before plan mode.** The plugin intercepts `EnterPlanMode` and
expects `superpowers:brainstorming` to have run first
(`skills/using-superpowers/SKILL.md`). Entering plan mode cold skips a step the
plugin assumes happened.

**The plugin registers exactly ONE hook — SessionStart** — which injects
`using-superpowers/SKILL.md` as context. There is no Stop hook and no
PreToolUse hook, so **nothing in superpowers enforces a gate**: running
`scripts/check.sh` before calling work done is on you, in every repo, always.
The plugin also never mentions ultracode; every ultracode rule above is local
policy, so it can conflict with nothing the plugin ships.
