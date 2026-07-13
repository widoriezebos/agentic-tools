# Design Obligation Plan Template

Status: reusable template

Use this structure for any design or redesign that will lead to implementation, benchmark validation, debugger
validation, or model-heavy canaries. Keep a filled copy in the project plan or ledger that owns the work; do not use
this template file itself as a live gate.

## Contract

- User or benchmark contract:
- Current failing artifact or trigger:
- Success criteria:
- Non-goals:
- Stop budget:

## Supported Diagnosis

- Anchor symptoms:
- Supported root cause:
- Exhausted mechanisms:
- Boundary or invariant that must change:
- Design-principles constraints:

## Design Obligations

Do not implement while critical/high rows lack owners, code targets, or focused test targets. Do not run expensive
validation while any critical/high row is `MISSING`, `PARTIAL`, `CONTRADICTED`, or `BLOCKED`.

Copy this table into the owning project plan or ledger and fill it there:

```markdown
| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| OBL-001 | CRITICAL | Owning design section or artifact | Required behavior stated as an invariant | Named class/component | Target code path or method | Focused test path or command | Runtime run id and artifact path, or READY_FOR_RUNTIME before first run | MISSING | Assign owner and implementation target |
```

## Design

- Owner sequence:
- Data flow:
- State and invariants:
- Failure modes:
- Observability:
- Cleanup and clean cutover:

## Implementation Steps

```markdown
| Step | Status | Obligation ids | Target files/components | Work | Verification |
| --- | --- | --- | --- | --- | --- |
| STEP-001 | TODO | OBL-001 | Target component | Implement the owner/invariant | Focused test name or command |
```

## Pre-Implementation Gate

- [ ] Every critical/high obligation has a named owner.
- [ ] Every critical/high obligation has a code target.
- [ ] Every critical/high obligation has a focused test target.
- [ ] No critical/high obligation is `MISSING` because the design forgot the owner.

## Pre-Benchmark Gate

Run the repository's obligation checker before benchmark, canary, debugger reproducer, or model-heavy validation when one
exists:

```bash
scripts/quality/assert-design-obligation-gate.sh --file <owning-plan-or-ledger.md>
```

- [ ] Every critical/high obligation is `DONE` or `READY_FOR_RUNTIME`.
- [ ] Every `READY_FOR_RUNTIME` obligation has owner, concrete code proof, and concrete test proof.
- [ ] Cycle contract names the runtime proof that will update each `READY_FOR_RUNTIME` row.
- [ ] Current state is recoverable by commit or named stash.

## Runtime Gate

After the first runtime validation:

- [ ] Update runtime proof for every relevant obligation.
- [ ] Convert every `READY_FOR_RUNTIME` row to `DONE`, `PARTIAL`, `CONTRADICTED`, or `BLOCKED`.
- [ ] Update the benchmark investigation ledger when applicable.
- [ ] Do not patch or rerun until contradicted obligations have a new cycle contract.

## Final Review Gate

Run a runtime-required gate before claiming implementation complete when runtime validation has happened:

```bash
scripts/quality/assert-design-obligation-gate.sh --runtime-required --file <owning-plan-or-ledger.md>
```

- [ ] Every critical/high obligation is `DONE`.
- [ ] Structured traces and execution logs contain enough state to explain the result.
- [ ] Final answer or report states any unresolved medium/low obligations.
