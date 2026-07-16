# Ideal Agent Harness

A portable operating system for engineering work done by agents (Claude Code, Devin CLI, Codex-style runtimes) together with humans, in one repository. It is a routing and enforcement system, not a rule book: a thin always-loaded contract, canonical guidance loaded per phase, triggered skills for high-risk workflows, deterministic scripts for binary requirements, and a receipts loop that tunes the whole thing from real use.

It is judged by shipped task outcomes — not by whether its documents were followed.

## Why It Exists

Agents working on real projects fail in predictable ways, and most of those failures are harness failures, not model failures:

- **Context bloat.** Instruction files grow with every incident until every task pays the cost of every rule ever written, and the important rules drown.
- **Rabbit holes.** Investigations loop on adjacent tweaks with no stop condition, burning hours or expensive runs without a novel fact.
- **Silent behavior drift.** Refactors and "cleanups" change behavior late and broadly, and green unit tests are mistaken for proof.
- **False completion.** "Done" is claimed from inference (code exists, tests pass) rather than observation (the change was driven end-to-end).
- **Forgotten lessons.** Corrections are applied once and repeated forever; session context dies and the next session re-derives everything.
- **Unreviewable output.** The human — the actual bottleneck — gets omnibus diffs and buried risk.
- **Prose mistaken for enforcement.** Hard requirements live as sentences the model may or may not weight, instead of scripts that fail.

The harness exists to convert each of those failure classes into either a routed judgment standard, a triggered workflow, or a deterministic check — while keeping the always-loaded footprint small (audited, capped, currently well under half the budget).

## What It Does

| Capability | Canonical owner |
| --- | --- |
| Always-on operating contract: inspect first, match the requested action, ambiguity ladder, escalation of human-reserved decisions, correction capture, completion duties | [`AGENTS.md`](AGENTS.md) |
| Single routing index: what to load, when | [`wow.md`](wow.md) |
| Project facts: commands, invariants, reserved decisions, external ownership | [`docs/project-rules.md`](docs/project-rules.md) (replaced per project) |
| Design judgment and priority order for consequential change | [`docs/design/design-principles.md`](docs/design/design-principles.md) |
| Completion gate: a 5-question default check on every change; a full obligation matrix only on explicit risk triggers | [`docs/design/design-obligation-gate.md`](docs/design/design-obligation-gate.md) |
| End-to-end verification: observed behavior as the only proof a change works | [`skills/verify/`](skills/verify/SKILL.md) |
| Behavior-preserving refactor mode: trusted baseline, tests-before-restructuring, risk-sized replayable batches, validation ladder, bounded failure handling | [`skills/refactor/`](skills/refactor/SKILL.md) |
| Investigation stop-loss: evidence-first diagnosis, cycle contracts, classifications, hard stop conditions | [`skills/take-a-step-back/`](skills/take-a-step-back/SKILL.md) |
| Delegation judgment: when to fan out to subagents, when not to, peer-agent coexistence | [`docs/orchestration.md`](docs/orchestration.md) |
| Human collaboration: reviewable increments, review-guide-first reports, correction capture, escalation shape | [`docs/collaboration.md`](docs/collaboration.md) |
| Session continuity: owned handoff notes for multi-session streams | [`plans/README.md`](plans/README.md) |
| Measurement and learning: per-task receipts, cadence-triggered retro, evidence-based tuning and pruning | [`docs/project-adaptation.md`](docs/project-adaptation.md) + `scripts/receipt.sh` |
| Specialist opt-ins (e.g. live Java/JDWP debugging) | [`optional-skills/`](optional-skills/) |

## How It Works

Five design bets, each enforced rather than merely stated:

1. **Progressive disclosure.** Only `AGENTS.md` + `wow.md` load on every task; everything else loads at the phase where it helps. `scripts/audit-harness.sh` fails the build if the always-loaded word count exceeds its cap.
2. **One rule, one home, one owner.** Every control has exactly one canonical document; other files link to it and may state the trigger, but never paraphrase the rule. Routing lives only in `wow.md`.
3. **Hard checks for hard requirements.** Binary properties are scripts, not prose. Prose that could be a script is a defect.
4. **Evidence before prose.** New instructions must pass a change gate: name the observed failure, show the model cannot infer it, find the owner, prefer executable enforcement. Task-local plans, ledgers, and incident notes never become global policy without deliberate promotion.
5. **The harness measures itself.** Every repo-changing task appends a receipt; a cadence check triggers a retro; retros change instructions through the change gate — with pruning weighted equal to adding.

Runtime portability is handled by splitting intent from mechanism: skills and docs are runtime-neutral; each skill ships per-runtime subagent profile templates under `skills/<name>/agents/` (`claude.md`, `devin/AGENT.md`, `openai.yaml`) that adopting projects copy into their runtime's profile location.

### Layout

```text
AGENTS.md            always-loaded contract (small, audited)
CLAUDE.md            runtime compatibility pointer to AGENTS.md
wow.md               the single routing index
docs/
  project-rules.md   project facts — replaced on adoption
  orchestration.md   delegation and peer-agent judgment
  collaboration.md   the human side: review, corrections, escalation
  project-adaptation.md  adoption steps + receipts-and-retro loop
  design/            design principles + completion/obligation gate
  examples/          worked examples (filled matrix, filled ledger)
skills/              triggered workflows: verify, refactor, take-a-step-back
optional-skills/     opt-in specialists (debug-java) — enabled per project
scripts/             deterministic checks (see below)
plans/               task-local state, handoff notes, receipts ledger
meta/                template maintenance + rationale — NOT copied to projects
```

### Scripts

| Script | Job |
| --- | --- |
| `scripts/validate-harness.sh` | Full self-check: audit, skill validation, routed assets, positive and negative fixture tests for every gate script |
| `scripts/audit-harness.sh` | Required files, no outside references, placeholder leakage, always-loaded word cap |
| `scripts/validate-skill.sh` | Skill frontmatter and naming rules |
| `scripts/assert-design-obligation-gate.sh` | Structure and declared state of an obligation matrix |
| `scripts/refactor-baseline.sh` | Trusted-baseline record/check for refactor mode (clean worktree, ancestry, cadence backstop) |
| `scripts/receipt.sh` | Task receipts, retro cadence check, retro marker |

Scripts check structure and declared state; they cannot prove a named test or receipt is truthful. That honesty gap is closed by the retro's human veto and by git history as cross-check.

## Using It in a Fresh Project

Canonical steps live in [`docs/project-adaptation.md`](docs/project-adaptation.md); the short version:

1. Copy the harness contents into the repository root — **excluding `meta/`**.
2. Replace `docs/project-rules.md` with verified facts: commands, invariants, reserved decisions, the refactor acceptance gate, delegation facts.
3. Enable optional skills only where they apply (move `optional-skills/debug-java` into `skills/` only for JVM repos).
4. Register subagent profiles for the runtimes in use: `skills/<name>/agents/claude.md` → `.claude/agents/<name>.md`, `skills/<name>/agents/devin/AGENT.md` → `.devin/agents/<name>/AGENT.md`.
5. Run `scripts/validate-harness.sh`, then a focused project build/test.
6. Record the template commit SHA you adopted from (a line in `docs/project-rules.md`) — it makes future migration a diff instead of archaeology.
7. Work normally. Each repo-changing task ends with the completion check, verification when runnable, and a receipt. Run the first retro after a handful of tasks instead of waiting for the cadence — early routing errors are the cheapest to fix.

Adopting in an **existing** repository — one that already has agent instructions, skills, prompts, or rule files — follows the reconciliation manual instead: [`docs/harness-reconciliation.md`](docs/harness-reconciliation.md). It inventories every instruction asset, dispositions each rule into its canonical owner or deletes it (with a ledger as the human's review guide), and cuts over with no parallel instruction sources left behind.

## Updating and Migrating

**Tuning an adopted harness (the normal path).** Instruction changes should come from the receipts-and-retro loop: patterns in receipts → proposals through the change gate → human veto → recorded retro. Corrections captured mid-task go straight to their owning document. Resist editing the contract from a single anecdote — that is the failure mode this harness exists to prevent.

**Pulling template updates into a project.** Diff against the recorded adoption SHA and apply the three-bucket rule — project-owned (never overwrite), template-owned (take upstream), merge deliberately (local deltas re-applied on top of new template text). The procedure is owned by [`docs/harness-reconciliation.md`](docs/harness-reconciliation.md); finish with `scripts/validate-harness.sh` and a retro entry recording the new template SHA.

**Changing the template itself.** Applies to this repository only: every addition answers the change gate in [`meta/harness-architecture.md`](meta/harness-architecture.md) (template repo only), keep/remove decisions are recorded in `meta/source-analysis.md`, structural claims must pass `scripts/validate-harness.sh`, and external critiques get a written disposition — implemented, deferred with a named revisit trigger, or rejected — rather than wholesale adoption.

## Status

Structurally validated end-to-end; behaviorally unproven until adopted — by design, the receipts-and-retro loop is how it earns (or loses) each of its rules in a real project. Provenance: distilled from production engineering repositories, agent-evaluation work, runtime-debugging practice, and reviewed against an independent external critique; sources and decisions are traceable in `meta/source-analysis.md` (template repo only).
