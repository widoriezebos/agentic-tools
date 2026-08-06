# Metasystem: the system to build a system

Every software system is really two systems. There is the system you ship —
the code, the tests, the running thing. And there is the system that builds
and evolves it: who may change what, what "done" means, which checks gate a
merge, when a human must decide, how lessons from one incident reach the next
task. In most repositories that second system is implicit — tribal knowledge,
review habits, prompt files, hope. The metasystem is that second system made
explicit, executable, and improvable: a small always-loaded contract,
guidance that loads only when needed, skills for high-risk workflows, scripts
that enforce every binary rule, supervision for anything that runs
unattended, and a receipts-and-retro loop that tunes all of it from real use.

The metasystem is judged by shipped results rather than by whether its
documents were followed.

## One builder per system: there is nothing generic here

A common misreading is that this is a general agent framework that "builds
software". It is not, and cannot be: **a metasystem instance is always the
builder of one specific system, and the pair is the unit that works.** What
this repository ships is the machinery every such pair shares — dispatch,
supervision, missions, gates, the retro loop — with deliberate empty sockets.
Adoption fills the sockets with the target system's own facts: its verified
commands and invariants in `docs/project-rules.md`, its reserved human
decisions, its budgets, its acceptance gates, its evidence root. The instance
that builds a task runner and the instance that builds a trading system share
machinery the way two factories share conveyor design; neither can do the
other's job, because the job is defined by the sockets, not the conveyor.

Both halves of the pair evolve, each with its own loop:

- **The product evolves** through gated work: tasks and unattended missions
  under signed contracts, fences, and completion gates that measure rather
  than trust.
- **The builder evolves** through evidence about itself. Day to day, that is
  the receipts-and-retro loop: every rule carries a testable expected effect
  and is kept, amended, or reverted against what the receipts show. And where
  it matters enough to measure whole, the builder is benchmarked: a fixed
  spec goes in, agents build it unattended, and a held-out grader scores the
  software while an extractor scores the builders' behaviour. Those graders
  and specs are themselves specific — a benchmark is built per kind of
  system, and its answer key is held outside the builder's reach — which is
  why the measuring kit lives beside a metasystem's source, never inside the
  payload.

The aim, in one sentence: **make building software governed by evidence, and
make improvements to the builder measurable instead of argued.** The end
state that matters is unattended competence with honest limits — an agent
crew that can carry real work through signed contracts and hard fences,
knows what it may never decide alone, reports what it measured rather than
what it believes, and leaves behind evidence a human or a grader can check.
The most useful single number such a pair produces is the distance between
the builders' own "done" and the held-out measurement of it.

This template's own source is developed by an instance of itself — the same
hooks, gates, supervision, and receipts that ship to every project run here
first. That is the first pair, and it is why the rules in this repository
tend to be scar tissue with a receipt attached rather than theory.

## Why it exists

Agents working on real projects fail in predictable ways, and most of those failures come from the metasystem around the agent rather than from the model:

- **Context bloat.** Instruction files grow with every incident until every task pays the cost of every rule ever written, and the important rules drown.
- **Rabbit holes.** Investigations loop on small variations of the same fix, burning hours or expensive runs without producing a new fact.
- **Silent behavior drift.** Refactors and cleanups change behavior without anyone noticing, while green unit tests get mistaken for proof.
- **False completion.** "Done" gets claimed because the code exists and tests pass, without anyone driving the change end to end.
- **Forgotten lessons.** Corrections are applied once and then repeated forever. Session context dies and the next session starts from scratch.
- **Unreviewable output.** The human reviewer gets one huge diff with the risky change buried in the middle.
- **Unsupervised runs.** A long job hangs in its tenth minute but reports "running" all night to a status-only watcher, or a healthy quiet run gets killed for silence.
- **Runaway spend.** Paid runs launch on estimates, failure-mode runs cost multiples of healthy ones, and the gap surfaces on the invoice.
- **Prose mistaken for enforcement.** Hard requirements live as sentences the model may or may not follow, instead of scripts that fail.

Each failure class has a named answer, and where the rule is binary, a script that enforces it:

| Failure | Addressed by | Enforced by |
| --- | --- | --- |
| Context bloat | A small always-loaded contract (`AGENTS.md`) with a single routing index (`wow.md`). Everything else loads at the phase where it helps, and new rules must pass the change gate | `scripts/audit-metasystem.sh` fails when the always-loaded word count exceeds its cap; the retro removes rules that cannot show their value |
| Rabbit holes | The take-a-step-back skill: every attempt gets a written contract with a budget, every result gets classified, and the stop-loss triggers end an investigation that stopped producing facts | `scripts/assert-stop-loss.sh` blocks new cycles once the ledger records a dead end, two no-progress cycles, or an exhausted cycle budget |
| Silent behavior drift | The refactor skill: a trusted baseline, tests before restructuring, replayable batches, and the project's acceptance gate as the only proof that behavior was preserved | `scripts/refactor-baseline.sh` blocks new batches on a dirty worktree, diverged history, or an overdue gate run |
| False completion | The verify skill (drive the change end to end and report the observed output) and the five-question completion check, with the obligation matrix for risky changes | `scripts/assert-design-obligation-gate.sh` refuses completion while critical obligations lack proof. A report that says "should work" is treated as a defect |
| Forgotten lessons | Correction capture (a correction updates the instructions in their one owning document) and handoff notes that carry unfinished work across sessions | Receipts record every correction, the retro reviews the pattern, and the instruction ledger holds every rule change with a testable expected effect |
| Unreviewable output | The collaboration rules: one intent per commit, mechanical churn separated from behavior change, and reports that start with the riskiest part | The human sends unreviewable diffs back; splitting them is the agent's job, and repeated offenses become retro findings |
| Unsupervised runs | The supervision rules in `docs/orchestration.md`: detached launches, a verified liveness signal, one watcher armed per session over every job the session can create, budgets that wind down instead of interrupting | `scripts/watch-background-jobs.sh` reports terminal, stale, capped and vanished jobs from a runner's job directory; `scripts/validate-metasystem.sh` exercises all four; remaining incidents land in receipts and `plans/known-issues.md` |
| Runaway spend | Budgets as project facts, spend measured from the provider's own records, and overage or a costlier resource tier as human-reserved decisions | No script can read an external invoice: the fence lives in `docs/project-rules.md`, overage requires an explicit ask, and the retro compares spend against receipts |
| Prose mistaken for enforcement | Binary rules become scripts, and the enforcement ships ready to wire in: a CI workflow and Claude Code hooks under `scripts/enforcement/` | `scripts/validate-metasystem.sh` runs positive and negative fixtures for the gate scripts, in the template and in adopted repositories |

## What it does

| Capability | Canonical owner |
| --- | --- |
| Always-on operating contract: inspect first, match the requested action, resolve ambiguity in a fixed order, escalate human-reserved decisions, capture corrections, completion duties | [`AGENTS.md`](AGENTS.md) |
| Single routing index: what to load, when | [`wow.md`](wow.md) |
| Project facts: commands, invariants, reserved decisions, budgets, external ownership | [`docs/project-rules.md`](docs/project-rules.md) (replaced per project) |
| Code and design standards: priority order, responsibility-driven ownership, domain-driven design for domain-heavy systems, self-documenting code | [`docs/design/design-principles.md`](docs/design/design-principles.md) |
| Completion gate: a five-question default check on every change; one full-suite run at declared milestones; a full obligation matrix only on explicit risk triggers | [`docs/design/design-obligation-gate.md`](docs/design/design-obligation-gate.md) |
| End-to-end verification: observed behavior as the only proof a change works | [`skills/verify/`](skills/verify/SKILL.md) |
| Adversarial design critique: findings-only critique, a materiality criterion that binds the critic, adjudication of every finding, and a loop that stops when critique stops changing what would be built | [`skills/design-critique/`](skills/design-critique/SKILL.md) |
| Behavior-preserving refactor mode: trusted baseline, tests before restructuring, risk-sized replayable batches, validation ladder, bounded failure handling | [`skills/refactor/`](skills/refactor/SKILL.md) |
| Improvement mode: improvement contract, frontier ledger with noise floor, single-mechanism experiments, anti-overfitting, stop conditions | [`skills/improve/`](skills/improve/SKILL.md) |
| Investigation stop-loss: evidence-first diagnosis, cycle contracts, classifications, hard stop conditions | [`skills/take-a-step-back/`](skills/take-a-step-back/SKILL.md) |
| Delegation judgment: when to fan out to subagents, when to avoid it, how delegated implementation is specified and reviewed, how long runs are supervised, how peer agents coexist | [`docs/orchestration.md`](docs/orchestration.md) |
| Human collaboration: reviewable increments, reports that lead with the review guide, correction capture, escalation shape | [`docs/collaboration.md`](docs/collaboration.md) |
| The human's guide: handing over work, reviewing, making corrections stick, running multiple agents, recurring duties | [`docs/working-with-agents.md`](docs/working-with-agents.md) |
| Session continuity: owned handoff notes for multi-session streams | [`plans/README.md`](plans/README.md) |
| Measurement and self-improvement: per-task receipts, cadence-triggered retro, instruction changes with testable expected effects that get reviewed and reverted like experiments | [`skills/retro/`](skills/retro/SKILL.md) and `scripts/receipt.sh` |
| Specialist opt-ins (for example live Java/JDWP debugging) | [`optional-skills/`](optional-skills/) |

## How it works

The day-to-day system is a set of **working modes**: implement, design, refactor, improve, take-a-step-back, verify. Each mode is a different promise about what "done" means, and each has its own failure modes and rules. The plain-English guide to all of them is [`docs/working-modes.md`](docs/working-modes.md). Start there.

Underneath the modes sit five design rules, each backed by a script or an explicit convention:

1. **Progressive disclosure.** Only `AGENTS.md` and `wow.md` load on every task; everything else loads at the phase where it helps. `scripts/audit-metasystem.sh` fails when the always-loaded word count exceeds its cap. Phase-loaded files such as `docs/project-rules.md` sit outside the cap and are bounded by review discipline instead. The honest per-task cost is larger than the audited pair: project rules, collaboration, and the completion gate load on nearly every repo-changing task, and design principles on most, so the audit also prints that effective common-path bundle as an uncapped report-only number, keeping the two metrics from being conflated.
2. **One rule, one home.** Every control has exactly one canonical document. Other files link to it and may state the trigger, but must not restate the rule. Routing lives only in `wow.md`. There are two declared exceptions: `docs/working-modes.md` restates rules in plain English for teaching, constants included, and this README restates rules and routing to pitch the metasystem to a first-time reader. Neither sets rules of its own, and both lose on any conflict.
3. **Hard checks for hard requirements.** Binary properties are scripts. A binary rule that exists only as prose is a defect.
4. **Evidence before rules.** New instructions must pass a change gate: name the observed failure, show the model cannot infer the rule on its own, find the owner, and prefer executable enforcement. Task-local plans, ledgers, and incident notes never become global policy without deliberate promotion.
5. **The metasystem measures itself.** Every repo-changing task appends a receipt. A cadence check triggers a retro. Retros change instructions through the change gate, and pruning carries the same weight as adding. Each adopted change records a testable expected effect in an instruction ledger, and the next retro reviews it: kept, amended, or reverted. Rule changes are experiments, and unsupported ones are removed by default.

Runtime portability is handled by separating intent from mechanism: skills and docs are runtime-neutral, and each skill ships per-runtime subagent profile templates under `skills/<name>/agents/` (`claude-profile.md`, `devin/AGENT.md`, `openai.yaml`) that adopting projects copy into their runtime's profile location.

### Layout

```text
AGENTS.md            always-loaded contract (small, audited)
CLAUDE.md            runtime compatibility pointer to AGENTS.md
wow.md               the single routing index
docs/
  project-rules.md   project facts, replaced on adoption
  working-modes.md   plain-English guide to all working modes
  working-with-agents.md  the human teammate's guide
  orchestration.md   delegation and peer-agent judgment
  collaboration.md   the human side: review, corrections, escalation
  project-adaptation.md  adoption steps plus the receipts-and-retro loop
  metasystem-reconciliation.md  manual for repos with existing instructions
  design/            design principles plus the completion gate
  examples/          worked examples (filled matrix, filled ledger)
skills/              triggered workflows: verify, design-critique, refactor, improve, retro, take-a-step-back
optional-skills/     opt-in specialists (debug-java), enabled per project
scripts/             deterministic checks and shipped enforcement configs
plans/               task-local state, handoff notes, standing ledgers
development/                template maintenance and rationale, never copied to projects
```

### Scripts

| Script | Job |
| --- | --- |
| `scripts/validate-metasystem.sh` | Full self-check: audit, skill validation, routed assets, positive and negative fixture tests for the gate scripts. Works in both the template and adopted repositories |
| `scripts/audit-metasystem.sh` | Required files, no outside references in metasystem files, placeholder leakage, always-loaded word cap |
| `scripts/validate-skill.sh` | Skill frontmatter and naming rules |
| `scripts/assert-design-obligation-gate.sh` | Structure and declared state of an obligation matrix |
| `scripts/refactor-baseline.sh` | Trusted-baseline record and check for refactor mode: clean worktree, ancestry, cadence backstop |
| `scripts/frontier.sh` | Best-known-state ledger for improvement mode. `record` refuses frontier regressions, `challenge` enforces the noise floor, and both refuse comparisons against a frontier older than its declared measurement window |
| `scripts/receipt.sh` | Task receipts, retro cadence check, comparable period stats, retro marker |
| `scripts/assert-stop-loss.sh` | Blocks new investigation cycles once the ledger records a dead end, two no-progress cycles, or an exhausted cycle budget |
| `scripts/enforcement/` | Shipped CI workflow and Claude Code hooks so the checks run without anyone remembering them |

Scripts check structure and declared state. They cannot prove that a named test or receipt is truthful. That gap is covered by the human veto at retro time and by git history as a cross-check.

## Using it in a fresh project

The canonical steps live in [`docs/project-adaptation.md`](docs/project-adaptation.md). The short version is three steps:

1. From the template checkout, run `scripts/adopt.sh <target> [--runtimes claude,devin,codex] [--enable debug-java]`. It exports the payload from the template's tracked HEAD, registers skills and subagent profiles for the selected runtimes, installs the shipped CI workflow and Claude Code hook, creates the gitignored `artifacts/` directory, and records the template SHA for future migrations. It refuses targets that already carry instruction assets; those follow the reconciliation manual below.
2. Fill `docs/project-rules.md` with verified facts: commands, invariants, reserved decisions, budgets, the refactor acceptance gate, delegation facts.
3. Run `scripts/validate-metasystem.sh` in the target; it must pass with zero placeholders. Then work normally: each repo-changing task ends with the completion check, verification when runnable, and a receipt. Run the first retro after a handful of tasks instead of waiting for the cadence. Early routing errors are the cheapest to fix.

For the engineers on the team, [`docs/working-with-agents.md`](docs/working-with-agents.md) is the manual: how to hand over work, what comes back to you, how to review, how to make corrections stick, and how to run several agents without collisions.

Adopting in an **existing** repository, one that already has agent instructions, skills, prompts, or rule files, follows the reconciliation manual instead: [`docs/metasystem-reconciliation.md`](docs/metasystem-reconciliation.md). It inventories every instruction asset, moves each rule into its canonical owner or deletes it (a ledger gives the human a review guide), and cuts over with no parallel instruction sources left behind.

## Updating and migrating

**Tuning an adopted metasystem (the normal path).** Instruction changes come from the receipts-and-retro loop: patterns in receipts, proposals through the change gate, human veto, recorded retro. Corrections captured mid-task go straight to their owning document. Resist editing the contract from a single anecdote. That is the failure mode this metasystem exists to prevent.

**Pulling template updates into a project.** Diff against the recorded adoption SHA and apply the three-bucket rule: project-owned files are never overwritten, template-owned files are taken from upstream, and shared files are merged deliberately with local changes re-applied on top. The procedure is owned by [`docs/metasystem-reconciliation.md`](docs/metasystem-reconciliation.md). Finish with `scripts/validate-metasystem.sh` and a retro entry recording the new template SHA.

**Changing the template itself.** Applies to this repository only: every addition answers the change gate (owned by [`docs/project-adaptation.md`](docs/project-adaptation.md), shipped with every project), keep-or-remove decisions are recorded in `development/source-analysis.md`, structural claims must pass `scripts/validate-metasystem.sh`, and external critiques get a written disposition: implemented, deferred with a named revisit trigger, or rejected with a reason.

## This template repository itself

This checkout is simply an adopted repository whose product is the template's
own source: its `plans/` are this project's plans, its gitignored `artifacts/`
is this project's live runtime state, and the same hooks and gates that ship
to every project run here first. What builds and measures the template lives
BESIDE it, never inside: the toplevel README explains the layout, and
`development/metasystem-inventory.md` classifies every path here as SHIPS,
PROJECT-STATE, or RUNTIME with the deciding rule for each.

## Status

The structure is validated end to end, and the loop has now run for real: the template repository develops itself under its own rules (its hooks, supervision, and receipts are live, not aspirational), a deterministic mission runner has completed unattended missions, and the measuring kit beside this template has graded an unattended build against a held-out battery. The receipts loop remains how each rule earns its keep or gets removed. The metasystem was distilled from production engineering repositories, agent-evaluation work, and runtime-debugging practice, and it has been reviewed against three independent external critiques. Sources and decisions are traceable in `development/source-analysis.md` (template repository only).
