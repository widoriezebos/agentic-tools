# Design Obligation Gate

Status: canonical companion protocol

This document defines the design-obligation workflow for this repository. It is intentionally stable and portable: copy
this file, or the template section at the bottom, when another project or agent needs the same behavior.

Incident ledgers, benchmark notes, and case-specific plans may contain live obligation matrices, but they are evidence
and work records. They are not the canonical source for the workflow.

## Purpose

The design standard in `docs/design/design-principles.md` says what good design means. This gate makes every important
design obligation explicit before implementation and before completion is claimed.

The gate prevents common false proofs:

- treating component existence as proof that the responsibility is owned;
- treating a passing test as proof when it does not map to a named obligation;
- treating a model prompt, selector, adapter, or coordinator as proof that the required boundary exists;
- treating a first runtime pass as completion before the runtime artifacts have been checked against the design.

## When This Is Required

Use a design-obligation matrix whenever any of these are true:

- a design or redesign will be implemented;
- a refactor changes ownership, boundaries, state, lifecycle, failure behavior, or observability;
- benchmark, debugger, canary, or model-heavy validation will be used to prove the design;
- a plan or design is being reviewed for implementation readiness;
- completion is being claimed for a design implementation.

Small local fixes that do not change a design boundary may not need a full matrix. If the change introduces a new owner,
stage, invariant, state transition, fallback path, model contract, or operational signal, it needs the gate.

## Required Matrix

Every owning plan or ledger must include a filled matrix with this exact header:

```markdown
| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
```

If a plan combines multiple external reference behaviors, the matrix must also contain a `CRITICAL` or `HIGH` obligation
row for the provider-neutral canonical Yoda contract that reconciles the combined behavior. That row must name the Yoda
owner, Yoda state/data shape, tool/provider/prompt contract, runtime trace contract, and the test/runtime proof that the
mixed upstream lessons compose as Yoda behavior.

Required column meanings:

- `Obligation id`: stable id used in implementation steps, tests, runtime evidence, and final review.
- `Severity`: `CRITICAL`, `HIGH`, `MEDIUM`, or `LOW`.
- `Design source`: the design section, principle, user requirement, benchmark contract, or artifact that creates the
  obligation.
- `Required behavior`: the invariant, boundary, state transition, failure behavior, data-flow guarantee, or observable
  outcome that must become true.
- `Owner`: the single component, class, module, or stage responsible for making the behavior true.
- `Code proof`: concrete implementation target, such as a class, method, module, or script.
- `Test proof`: focused test path, test name, or command that proves the obligation at the owning level.
- `Runtime proof`: run id, execution log, benchmark artifact, trace, screenshot, or explicit reason runtime proof is not
  applicable.
- `Status`: one of the allowed statuses below.
- `Next action`: the next concrete action needed to move the obligation forward.

Severity definitions:

- `CRITICAL`: required for correctness, truthfulness, provenance, safety, security, or pass/fail validity.
- `HIGH`: required to prevent repeated stop-loss loops, major cost waste, opaque failure, or boundary drift.
- `MEDIUM`: important for maintainability, diagnostics, robustness, or future change cost.
- `LOW`: cleanup, polish, or secondary consistency work.

Status definitions:

- `MISSING`: no owner, implementation target, or real implementation exists.
- `PARTIAL`: some code exists, but the full obligation is not proven.
- `READY_FOR_RUNTIME`: implemented and focused-test proven, but runtime proof has not been collected yet.
- `DONE`: implemented, focused-test proven, and runtime-proven where runtime proof is applicable.
- `CONTRADICTED`: runtime evidence disproves the obligation.
- `BLOCKED`: the obligation cannot move without a named blocker and user-visible decision.

## Pre-Implementation Gate

Before implementation starts:

- every `CRITICAL` and `HIGH` obligation must have a named owner;
- every `CRITICAL` and `HIGH` obligation must have a concrete code target;
- every `CRITICAL` and `HIGH` obligation must have a focused test target;
- no `CRITICAL` or `HIGH` obligation may be vague about the boundary, invariant, or state transition it proves;
- implementation steps must reference the obligation ids they satisfy.
- any combined external reference design must include a provider-neutral canonical Yoda contract obligation before
  implementation.

Do not implement while a `CRITICAL` or `HIGH` obligation lacks an owner, code target, or focused test target.

## Pre-Validation Gate

Before any benchmark, canary, debugger reproducer, or model-heavy validation used to prove a design:

- every `CRITICAL` and `HIGH` obligation must be `DONE` or `READY_FOR_RUNTIME`;
- every `READY_FOR_RUNTIME` obligation must have owner, code proof, and focused test proof;
- the plan must name the runtime artifact that will prove or contradict each `READY_FOR_RUNTIME` row;
- the current worktree state must be recoverable by commit or named stash when the validation is expensive;
- the cycle contract must state what result changes each `READY_FOR_RUNTIME` row to `DONE`, `PARTIAL`,
  `CONTRADICTED`, or `BLOCKED`.

When the repository checker is available, run:

```bash
scripts/quality/assert-design-obligation-gate.sh --file <owning-plan-or-ledger.md>
```

`READY_FOR_RUNTIME` permits one first runtime validation. It is not a completion status.

## Runtime Gate

After the first runtime validation:

- inspect the primary runtime artifacts before patching or rerunning;
- update runtime proof for every relevant obligation;
- convert every `READY_FOR_RUNTIME` `CRITICAL` or `HIGH` row to `DONE`, `PARTIAL`, `CONTRADICTED`, or `BLOCKED`;
- record exact run ids and artifact paths when benchmark or execution-log artifacts exist;
- if runtime contradicts a `CRITICAL` or `HIGH` obligation, stop and redesign or patch only under a new cycle contract.

Runtime evidence must prove the designed owner sequence actually ran and that the required boundary, invariant, state
transition, failure behavior, or observable signal became true. A better-looking result is not enough.

## Completion Gate

Do not call a design implementation complete while any `CRITICAL` or `HIGH` obligation is `MISSING`, `PARTIAL`,
`READY_FOR_RUNTIME`, `CONTRADICTED`, or `BLOCKED`.

Before claiming completion after runtime validation, run the runtime-required gate when the checker is available:

```bash
scripts/quality/assert-design-obligation-gate.sh --runtime-required --file <owning-plan-or-ledger.md>
```

Completion means:

- every `CRITICAL` and `HIGH` obligation is `DONE`;
- code proof points to the owning implementation, not just a caller or coordinator;
- test proof maps to the obligation id and proves positive behavior, not only negative rejection;
- runtime proof cites the exact artifact, or clearly states why runtime proof is not applicable;
- unresolved `MEDIUM` and `LOW` obligations are listed in the final report or follow-up plan.

## Copyable Template

Use this section when creating a design plan.

```markdown
## Design Obligation Gate

### Contract

- User or benchmark contract:
- Success criteria:
- Non-goals:
- Runtime validation needed:
- Stop budget, if validation is expensive:

### Obligation Matrix

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| OBL-001 | CRITICAL | Design section or requirement | Required behavior stated as an invariant | Named owner | Target class/module/method | Focused test path or command | Runtime artifact path, or not applicable with reason | MISSING | Assign owner, code target, and test target |
| OBL-CANONICAL-YODA | HIGH | External reference design canonicalization | Combined external reference behavior is reconciled into one provider-neutral canonical Yoda contract before implementation | Named Yoda owner | Target class/module/method or plan section | Focused contract/parity test command | Runtime artifact path, or not applicable with reason | MISSING | Define Yoda owner, state shape, tool/provider/prompt contract, and trace contract |

### Implementation Steps

| Step | Status | Obligation ids | Target files/components | Work | Verification |
| --- | --- | --- | --- | --- | --- |
| STEP-001 | TODO | OBL-001 | Target owner | Implement the invariant or boundary | Focused test command |

### Pre-Implementation Gate

- [ ] Every critical/high obligation has a named owner.
- [ ] Every critical/high obligation has a concrete code target.
- [ ] Every critical/high obligation has a focused test target.
- [ ] Implementation steps reference obligation ids.

### Pre-Validation Gate

- [ ] Every critical/high obligation is `DONE` or `READY_FOR_RUNTIME`.
- [ ] Every `READY_FOR_RUNTIME` row has owner, code proof, and test proof.
- [ ] Runtime proof path is named for each `READY_FOR_RUNTIME` row.
- [ ] Current state is recoverable by commit or named stash if validation is expensive.
- [ ] `scripts/quality/assert-design-obligation-gate.sh --file <owning-plan-or-ledger.md>` passes when available.

### Runtime Gate

- [ ] Runtime artifacts have been inspected.
- [ ] Every `READY_FOR_RUNTIME` critical/high row is now `DONE`, `PARTIAL`, `CONTRADICTED`, or `BLOCKED`.
- [ ] Contradicted or partial obligations have a new cycle contract before any patch or rerun.

### Completion Gate

- [ ] Every critical/high obligation is `DONE`.
- [ ] Runtime proof is cited where applicable.
- [ ] `scripts/quality/assert-design-obligation-gate.sh --runtime-required --file <owning-plan-or-ledger.md>` passes
      when runtime validation happened and the checker is available.
- [ ] Remaining medium/low obligations are listed as follow-up work.
```
