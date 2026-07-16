# Design Obligation Gate

## Default Completion Check (every change)

Answer inline before calling any change complete; no file or matrix is required:

1. Is the requested contract met, stated as the observable outcome?
2. Does each new or moved behavior have one named owner?
3. What focused verification ran, and what did it show? (`skills/verify/SKILL.md` for runnable surfaces.)
4. What happens on failure, timeout, or bad input in the changed path?
5. What remains unverified, and is that stated in the report?

If any answer is weak, fix or report it. Escalate to the full matrix only on the triggers below.

## Full Matrix (risky changes only)

Use a matrix when a change introduces or moves an owner, boundary, invariant, lifecycle, state transition, failure behavior, model/tool contract, or operational signal; or when expensive evidence will prove it. If no trigger applies, do not build a matrix — the default check above is the whole gate. A filled example: `docs/examples/design-obligation-matrix.md`.

```markdown
| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
```

Allowed severity: `CRITICAL`, `HIGH`, `MEDIUM`, `LOW`.

Allowed status:

- `MISSING`: owner or proof target is absent.
- `PARTIAL`: implementation/proof is incomplete.
- `READY_FOR_RUNTIME`: code and focused tests pass; one named runtime proof remains.
- `DONE`: required code, tests, and applicable runtime proof exist.
- `CONTRADICTED`: runtime evidence disproves the obligation.
- `BLOCKED`: a named external decision or state prevents progress.

## Gates

- Before implementation: every critical/high row has a specific owner, code target, and focused test target; steps reference obligation ids.
- Before expensive validation: critical/high rows are `DONE` or `READY_FOR_RUNTIME`; the state is recoverable; each run has a question, expected signal, budget, and stop condition.
- After runtime: inspect primary artifacts and replace every relevant `READY_FOR_RUNTIME` status with an evidence-backed result before patching or rerunning.
- Before completion: every critical/high row is `DONE`; unresolved medium/low rows are reported.

Run:

```bash
scripts/assert-design-obligation-gate.sh --file plans/<plan>.md
scripts/assert-design-obligation-gate.sh --runtime-required --file plans/<plan>.md
```

The matrix is semantic work; the script checks structure and declared state. A passing script does not prove that a named owner or test is truthful.
