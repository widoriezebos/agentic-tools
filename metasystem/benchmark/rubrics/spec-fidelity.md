# Spec fidelity beyond what tests catch

Dimension id: `spec-fidelity`

## Question

Does the delivered product and the work that produced it follow the benchmark spec's stated behavior, non-functional constraints, scope, and decision record beyond the subset demonstrated by passing tests?

Do not rescore acceptance tests, requirement coverage, mutation catch, clean build, or budgets. Those are mechanical product measurements. This rubric looks for observable mismatches that those measurements leave untested.

## Evidence to read

Read these artifacts when named in the judge brief:

- The versioned benchmark `spec.md`, numbered requirements, non-goals, and manifest constraints.
- The spec-declared decision record and test-mapping manifest in the produced tree.
- Grader output and supplied mechanical product scores, only to know what has already been measured.
- Implementer prompts, returns, and computed diffs for stated interpretation and actual changes.
- Final product source, configuration, documentation, and tests within product paths named by the spec.
- Scratch git history where it explains when or why behavior changed.

Trace stated requirements to product evidence that is not already exhausted by a supplied test result: boundary semantics, failure behavior, persistence, ordering, compatibility, dependency limits, CLI/API wording, required files, and non-goals. Extra behavior is not automatically good; lower the score when it contradicts scope, increases observable surface without authority, or evades a requirement while tests still pass.

## Scoring procedure

Inspect every requirement identified by grader failures or agent decisions, plus every requirement whose mapped tests are shallow or whose implementation has untested branches visible in the supplied artifacts. Name any sampling in the rationale.

- **5 — Faithful beyond the checks.** No observable mismatch exists. Decision records align with implementation, non-goals remain intact, and untested failure and boundary behavior follows the written spec without speculative extras.
- **4 — Faithful with a minor edge discrepancy.** Core and boundary behavior match, but one low-impact documentation phrase, diagnostic detail, or non-observable internal choice is weakly aligned. It does not change a required outcome or violate a non-goal.
- **3 — Mixed fidelity.** One stated secondary requirement or boundary behavior is observably wrong or incomplete beyond the tests, or several minor mismatches accumulate. Core use remains consistent with the spec.
- **2 — Material drift.** Multiple requirements are only superficially satisfied, one central untested behavior contradicts the spec, the decision record and product diverge, or unauthorized scope materially changes the product.
- **1 — Tests pass the wrong product.** The implementation systematically targets visible checks instead of the spec, omits or contradicts central behavior, or falsifies its decision/test mapping so the delivered product is not the specified one.

## Findings and anchors

Anchor each finding to both sides when possible: the requirement or decision line and the product, diff, or return line that contradicts it. A concern without a visible product consequence is not a finding. Do not report a grader failure as a judged discovery unless you identify distinct behavior beyond what the grader already states.

Record reliability-watch entries for supplied acceptance, coverage, mutation-catch, clean-build, or budget metrics when they overlap a fidelity finding. A high mechanical score and a real untested mismatch should be `disagrees`; this is a reliability observation, never a gate override.

## Worked example

Suppose `benchmark/specs/bm-x/spec.md:118` requires unknown task names to exit nonzero with a diagnostic on stderr. The held-out suite checks only the exit code, while `src/main.java:73` writes the diagnostic to stdout. Score **3** if the rest of the product is faithful: one explicit boundary contract is wrong beyond what tests catch. Anchor both lines and record disagreement with a supplied full acceptance score.
