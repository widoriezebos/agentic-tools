# Design Obligation Gate

## Default Completion Check (every change)

Answer inline before calling any change complete; no file or matrix is required:

1. Is the requested contract met, stated as the observable outcome?
2. Does each new or moved behavior have one named owner?
3. What focused verification ran, and what did it show? (`skills/verify/SKILL.md` for runnable surfaces.)
4. What happens on failure, timeout, or bad input in the changed path?
5. What remains unverified, and is that stated in the report?

If any answer is weak, fix or report it. Escalate to the full matrix only on the triggers below.

## Design Completeness (every design)

A design is complete enough to critique or to build only when it names **step 1** — what exists and is usable after one slice — and lists what it defers, each deferred item with the step-1 field or home it will build on. A design that answers a large request with the whole answer is not complete; it is the design of the cathedral, and R-121 (2026-09-22) rules that the smallest thing that works is designed and built first, and that iterating is a separate decision taken after step 1 has value. The critique loop's first question checks this (`skills/design-critique/SKILL.md`, Step 1 Before Anything), and a finding is material there only if it changes what step 1 builds.

## A design is a record

A design a seat writes during a goal is a record, not a file in a directory: it declares what it is in a head and names the goal it is for. Without that head it is read by nothing — not the resolver, not the goal page, not the Partner. The head is the first thing after the title, and the blank line between them is part of the grammar:

```markdown
# What is being designed

- Kind: design
- Id: 01K5R2H3ZQ8V6M7N9P0A1B2C3D
- Status: draft
- Goals: refund-worker
```

`Id` comes from `metasystem project id`, is unique across the project, and never changes, so the file may be renamed or moved. `Status` is `draft` while the design is being written and critiqued, `accepted` once a human has accepted it, `done` when the work it designed shipped, `superseded` when another design replaced it. `Goals` names the ledger goal the design is for, by the id the goal file carries; a design about the project as a whole carries no `Goals` line at all, and a record of the intent or doctrine kind may never carry one.

The home is `plans/designs/` **under the resolved state root**, subdirectories allowed. In the self-hosted layout that is the installation, `metasystem/plans/designs/`, with the checkout root's own `plans/designs/` read as a second home; in an adopted installation it is the application's own repository root, `plans/designs/`. It is never `vendor/metasystem/plans/designs/`: the state root of an adopted installation is the application, so a design written beneath the installation is read by nothing.

Before calling a design done, run the boundary rather than trusting the file just written:

```bash
metasystem project design-of --root . --goal <goal id>
```

It prints the id, status and path of every design record whose `Goals` names that goal, and refuses when there is none. The design-critique skill runs it first and stops on its refusal (`skills/design-critique/SKILL.md`): a critique of a design the resolver cannot find is a critique of a document that will not be found again.

`metasystem project check` reads every home and refuses a duplicate id, a head missing a required key, an unknown kind or status, and a `Goals` line naming a goal the ledger does not have. The fast gate runs it (`scripts/agents/go-gate.sh`), in an adopted installation from the installed executable, so a failing check blocks every delivery that consumes that gate's retained proof, an unrelated code change included. The design homes, the decision home, the intent and doctrine books and the question register are declared inputs of that group in `testing.json`, so a change to any of them invalidates the retained proof instead of inheriting it.

## Milestone Check (declared milestones only)

At a declared milestone (merging to the integration branch, closing a plan, ending a multi-session stream), recompute the shared testing contract from the actual goal, destination policy and whole-project candidate. Run its required selection once in your own environment and consume the sufficient retained result with `test verify`, in addition to the focused checks above. A low-risk milestone may finish with standard evidence; deep and cadence groups run only when the contract requires them, while the standing validator remains the broader backstop. A delegate's green run proves its environment, not yours. This named question justifies the selected expense; there is no blanket full-suite prerequisite between milestones.

## Full Matrix (risky changes only)

Use a matrix when a change introduces or moves an owner, boundary, invariant, lifecycle, state transition, failure behavior, model/tool contract, or operational signal; or when expensive evidence will prove it. If no trigger applies, do not build a matrix. For a single change, the default check above is the whole gate. A filled example: `docs/examples/design-obligation-matrix.md`.

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
bin/metasystem validate design-obligations --file plans/<plan>.md
bin/metasystem validate design-obligations --runtime-required --file plans/<plan>.md
```

The matrix is semantic work; the script checks structure and declared state. A passing script does not prove that a named owner or test is truthful.
