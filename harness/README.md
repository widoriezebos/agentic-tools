# Ideal Agent Harness

A portable set of working rules for repositories where agents (Claude Code, Devin CLI, Codex and similar) and humans build software together. It consists of a small always-loaded contract, guidance that loads only when needed, skills for high-risk workflows, scripts that enforce the binary rules, and a receipts loop that tunes everything from real use.

The harness is judged by shipped results rather than by whether its documents were followed.

## Why it exists

Agents working on real projects fail in predictable ways, and most of those failures come from the harness around the agent rather than from the model:

- **Context bloat.** Instruction files grow with every incident until every task pays the cost of every rule ever written, and the important rules drown.
- **Rabbit holes.** Investigations loop on small variations of the same fix, burning hours or expensive runs without producing a new fact.
- **Silent behavior drift.** Refactors and cleanups change behavior without anyone noticing, while green unit tests get mistaken for proof.
- **False completion.** "Done" gets claimed because the code exists and tests pass, without anyone driving the change end to end.
- **Forgotten lessons.** Corrections are applied once and then repeated forever. Session context dies and the next session starts from scratch.
- **Unreviewable output.** The human reviewer gets one huge diff with the risky change buried in the middle.
- **Prose mistaken for enforcement.** Hard requirements live as sentences the model may or may not follow, instead of scripts that fail.

For each of these failure classes the harness provides a guidance document, a triggered skill, or a deterministic check, while keeping the always-loaded footprint small. An audit script fails when the footprint grows past its word budget.

## What it does

| Capability | Canonical owner |
| --- | --- |
| Always-on operating contract: inspect first, match the requested action, resolve ambiguity in a fixed order, escalate human-reserved decisions, capture corrections, completion duties | [`AGENTS.md`](AGENTS.md) |
| Single routing index: what to load, when | [`wow.md`](wow.md) |
| Project facts: commands, invariants, reserved decisions, external ownership | [`docs/project-rules.md`](docs/project-rules.md) (replaced per project) |
| Design judgment and priority order for consequential change | [`docs/design/design-principles.md`](docs/design/design-principles.md) |
| Completion gate: a five-question default check on every change; a full obligation matrix only on explicit risk triggers | [`docs/design/design-obligation-gate.md`](docs/design/design-obligation-gate.md) |
| End-to-end verification: observed behavior as the only proof a change works | [`skills/verify/`](skills/verify/SKILL.md) |
| Behavior-preserving refactor mode: trusted baseline, tests before restructuring, risk-sized replayable batches, validation ladder, bounded failure handling | [`skills/refactor/`](skills/refactor/SKILL.md) |
| Improvement mode: improvement contract, frontier ledger with noise floor, single-mechanism experiments, anti-overfitting, stop conditions | [`skills/improve/`](skills/improve/SKILL.md) |
| Investigation stop-loss: evidence-first diagnosis, cycle contracts, classifications, hard stop conditions | [`skills/take-a-step-back/`](skills/take-a-step-back/SKILL.md) |
| Delegation judgment: when to fan out to subagents, when to avoid it, how peer agents coexist | [`docs/orchestration.md`](docs/orchestration.md) |
| Human collaboration: reviewable increments, reports that lead with the review guide, correction capture, escalation shape | [`docs/collaboration.md`](docs/collaboration.md) |
| The human's guide: handing over work, reviewing, making corrections stick, running multiple agents, recurring duties | [`docs/working-with-agents.md`](docs/working-with-agents.md) |
| Session continuity: owned handoff notes for multi-session streams | [`plans/README.md`](plans/README.md) |
| Measurement and self-improvement: per-task receipts, cadence-triggered retro, instruction changes with testable expected effects that get reviewed and reverted like experiments | [`skills/retro/`](skills/retro/SKILL.md) and `scripts/receipt.sh` |
| Specialist opt-ins (for example live Java/JDWP debugging) | [`optional-skills/`](optional-skills/) |

## How it works

The day-to-day system is a set of **working modes**: implement, design, refactor, improve, take-a-step-back, verify. Each mode is a different promise about what "done" means, and each has its own failure modes and rules. The plain-English guide to all of them is [`docs/working-modes.md`](docs/working-modes.md). Start there.

Underneath the modes sit five design rules, each backed by a script or an explicit convention:

1. **Progressive disclosure.** Only `AGENTS.md` and `wow.md` load on every task; everything else loads at the phase where it helps. `scripts/audit-harness.sh` fails when the always-loaded word count exceeds its cap. Phase-loaded files such as `docs/project-rules.md` sit outside the cap and are bounded by review discipline instead.
2. **One rule, one home.** Every control has exactly one canonical document. Other files link to it and may state the trigger, but must not restate the rule. Routing lives only in `wow.md`. There is one declared exception: `docs/working-modes.md` restates rules in plain English for teaching, constants included. It sets no rules of its own and loses on any conflict.
3. **Hard checks for hard requirements.** Binary properties are scripts. A binary rule that exists only as prose is a defect.
4. **Evidence before rules.** New instructions must pass a change gate: name the observed failure, show the model cannot infer the rule on its own, find the owner, and prefer executable enforcement. Task-local plans, ledgers, and incident notes never become global policy without deliberate promotion.
5. **The harness measures itself.** Every repo-changing task appends a receipt. A cadence check triggers a retro. Retros change instructions through the change gate, and pruning carries the same weight as adding. Each adopted change records a testable expected effect in an instruction ledger, and the next retro reviews it: kept, amended, or reverted. Rule changes are experiments, and unsupported ones are removed by default.

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
  harness-reconciliation.md  manual for repos with existing instructions
  design/            design principles plus the completion gate
  examples/          worked examples (filled matrix, filled ledger)
skills/              triggered workflows: verify, refactor, improve, retro, take-a-step-back
optional-skills/     opt-in specialists (debug-java), enabled per project
scripts/             deterministic checks and shipped enforcement configs
plans/               task-local state, handoff notes, standing ledgers
meta/                template maintenance and rationale, never copied to projects
```

### Scripts

| Script | Job |
| --- | --- |
| `scripts/validate-harness.sh` | Full self-check: audit, skill validation, routed assets, positive and negative fixture tests for the gate scripts. Works in both the template and adopted repositories |
| `scripts/audit-harness.sh` | Required files, no outside references in harness files, placeholder leakage, always-loaded word cap |
| `scripts/validate-skill.sh` | Skill frontmatter and naming rules |
| `scripts/assert-design-obligation-gate.sh` | Structure and declared state of an obligation matrix |
| `scripts/refactor-baseline.sh` | Trusted-baseline record and check for refactor mode: clean worktree, ancestry, cadence backstop |
| `scripts/frontier.sh` | Best-known-state ledger for improvement mode. `record` refuses frontier regressions; `challenge` enforces the noise floor |
| `scripts/receipt.sh` | Task receipts, retro cadence check, comparable period stats, retro marker |
| `scripts/enforcement/` | Shipped CI workflow and Claude Code hooks so the checks run without anyone remembering them |

Scripts check structure and declared state. They cannot prove that a named test or receipt is truthful. That gap is covered by the human veto at retro time and by git history as a cross-check.

## Using it in a fresh project

The canonical steps live in [`docs/project-adaptation.md`](docs/project-adaptation.md). The short version:

1. Copy the harness contents into the repository root, excluding `meta/` and the template's own `plans/receipts.log`.
2. Replace `docs/project-rules.md` with verified facts: commands, invariants, reserved decisions, the refactor acceptance gate, delegation facts.
3. Enable optional skills only where they apply (move `optional-skills/debug-java` into `skills/` only for JVM repos).
4. Register the skills with the runtimes (for example `skills/<name>/` to `.claude/skills/<name>/` so auto-triggering works), then the subagent profiles: `skills/<name>/agents/claude-profile.md` to `.claude/agents/<name>.md`, `skills/<name>/agents/devin/AGENT.md` to `.devin/agents/<name>/AGENT.md`.
5. Run `scripts/validate-harness.sh`, then a focused project build and test. Wire the checks into CI using the shipped `scripts/enforcement/` files.
6. Record the template commit SHA you adopted from (a line in `docs/project-rules.md`) so future migrations can diff against it.
7. Work normally. Each repo-changing task ends with the completion check, verification when runnable, and a receipt. Run the first retro after a handful of tasks instead of waiting for the cadence. Early routing errors are the cheapest to fix.

For the engineers on the team, [`docs/working-with-agents.md`](docs/working-with-agents.md) is the manual: how to hand over work, what comes back to you, how to review, how to make corrections stick, and how to run several agents without collisions.

Adopting in an **existing** repository, one that already has agent instructions, skills, prompts, or rule files, follows the reconciliation manual instead: [`docs/harness-reconciliation.md`](docs/harness-reconciliation.md). It inventories every instruction asset, moves each rule into its canonical owner or deletes it (a ledger gives the human a review guide), and cuts over with no parallel instruction sources left behind.

## Updating and migrating

**Tuning an adopted harness (the normal path).** Instruction changes come from the receipts-and-retro loop: patterns in receipts, proposals through the change gate, human veto, recorded retro. Corrections captured mid-task go straight to their owning document. Resist editing the contract from a single anecdote. That is the failure mode this harness exists to prevent.

**Pulling template updates into a project.** Diff against the recorded adoption SHA and apply the three-bucket rule: project-owned files are never overwritten, template-owned files are taken from upstream, and shared files are merged deliberately with local changes re-applied on top. The procedure is owned by [`docs/harness-reconciliation.md`](docs/harness-reconciliation.md). Finish with `scripts/validate-harness.sh` and a retro entry recording the new template SHA.

**Changing the template itself.** Applies to this repository only: every addition answers the change gate (owned by [`docs/project-adaptation.md`](docs/project-adaptation.md), shipped with every project), keep-or-remove decisions are recorded in `meta/source-analysis.md`, structural claims must pass `scripts/validate-harness.sh`, and external critiques get a written disposition: implemented, deferred with a named revisit trigger, or rejected with a reason.

## Status

The structure is validated end to end. Real-world behavior is unproven until a project adopts it; the receipts loop is how each rule earns its keep or gets removed. The harness was distilled from production engineering repositories, agent-evaluation work, and runtime-debugging practice, and it has been reviewed against three independent external critiques. Sources and decisions are traceable in `meta/source-analysis.md` (template repository only).
