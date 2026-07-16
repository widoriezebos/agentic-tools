# Source Analysis and Decisions

## Evidence Reviewed

- Repository instruction sets, portable working indexes, canonical design standards, obligation matrices, experiment ledgers, execution-log guidance, refactor/frontier controls, and recent engineering histories.
- Stateful knowledge-system work covering minimum-complexity gates, semantic-quality journals, recovery ownership, and bounded experiments.
- Model-backed document-processing work covering truth-vs-output separation, anti-overfitting, stop-loss ledgers, clean rollback, and exact-state validation.
- Descartes repository instructions and Java debug skill covering debugger lifecycle, async breakpoint orchestration, source/artifact parity, causal proof, and cleanup.
- Supplied transcript: harness mapping; blame the right layer; one rule/home/owner; phase-specific loading; hard checks for hard requirements; adapt to model and product; record receipts.

The repositories were treated as evidence, not as modules to concatenate. Recent histories show repeated contract/plan checkpoints, rollback of falsified experiments, exact-frontier preservation, recovery ownership work, debugger lifecycle fixes, and removal of obsolete paths. Those patterns carry more weight than isolated prose additions.

## Kept

| Practice | Why it survived | Portable owner |
| --- | --- | --- |
| Inspect before concluding or editing | Prevents stale assumptions and wrong-layer patches | `AGENTS.md` |
| One owner, honest boundaries, explicit state/failure | Repeatedly explains successful redesign and diagnosability | Design principles |
| Obligation-to-code/test/runtime traceability | Prevents component-exists and one-green-run false proof | Obligation gate |
| Focused proof before expensive proof | Improves diagnosis and cost control | Root contract + step-back skill |
| Cycle contracts, classifications, stop-loss | Stops adjacent tweaks and repeated expensive failures | Step-back skill |
| Current-consumer/necessity gate | Prevents zero-consumer architecture rabbit holes | Design + step-back skill |
| Recoverable checkpoints and exact-state proof | Preserves learning and prevents stale artifact claims | Step-back skill; project-specific frontier extensions |
| Source/artifact parity and causal-chain debugging | Prevents stale bytecode and correlation-as-root-cause errors | Debug skill |
| Cursor-based async debugger orchestration and cleanup | Prevents deadlocks, stale events, and shared-session damage | Debug skill |
| Direct truth separated from software output | Generalizes to goldens, fixtures, expected outputs, and evaluations | Testing/evaluation guidance in design and plans |
| Exceptions remain diagnosable | Stack trace plus structured context matters operationally | Project rules/design standard |
| Clean cutovers and deletion | Avoids dual primary behavior and permanent compatibility drag | Design principles |
| Refactor mode: trusted baseline, tests-before-refactor, risk-sized replayable clusters, validation ladder, bounded fix attempts | Repeatedly prevented silent behavior drift and unbounded failed-refactor spirals in the source repository | Refactor skill + `scripts/refactor-baseline.sh` |

## Removed or Narrowed

| Source pattern | Decision |
| --- | --- |
| Large repository-specific AGENTS runbooks | Move commands and domain rules to `docs/project-rules.md`; move specialist methods to skills/references. |
| Duplicate policies in root instructions, compatibility files, indexes, and skills | `AGENTS.md` owns the contract; `wow.md` routes; compatibility files link. |
| Repository-specific benchmark scores, frontier scripts, and commit cadence | Partially reversed by user decision: the portable invariants (trusted baseline, cadence backstop, cheapest-rejection-first gating, bounded fix attempts) were promoted into the refactor skill and `scripts/refactor-baseline.sh`. The concrete gate — benchmark cases, canary floors, memory modes, tags — remains excluded and is declared per project in `docs/project-rules.md`. |
| Document formats, fixture vocabulary, locale rules, expected-output schemas, and provider rules | Exclude domain terms. Retain the generic truth-vs-output and anti-overfit lessons. |
| Repository-specific workspace APIs, build profiles, UI launchers, and retrieval-lane rules | Keep only in each project's rules, not the template. |
| Domain-specific semantic scoring and learning-journal vocabulary | Retain experiment journaling and truth qualification as optional practices, not global rules. |
| Descartes full tool catalog and exhaustive troubleshooting detail | Keep critical debugger orchestration in `SKILL.md`; load operational detail from one-hop references. |
| Universal builder thresholds, deprecation bans, framework styling | Do not universalize. These are language/project policy, not harness invariants. |
| A new prose rule for every miss | Require evidence, canonical ownership, loading decision, and preferably executable enforcement. |
| Model-specific prompt recipes | Keep outcomes and checks stable; tune adapters/skills only from repeated product-specific evidence. |

## Ideal Harness Judgment

The ideal harness is not the union of best rules. It is a routing and enforcement system: a thin stable contract, one canonical design standard, triggered high-risk workflows, task-local evidence, and deterministic locks. Rich context remains available, but arrives only when useful.
