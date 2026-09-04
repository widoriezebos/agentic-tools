---
name: code-critique
description: Review an implementation against its brief and computed diff before attacking it adversarially, using an implementation-specific materiality test and a bounded correction loop. Use after code has been implemented from a brief or accepted design, when adjudicating implementation-review findings, or before certifying delegated implementation. Do not use to critique a design before implementation (use design-critique) or to diagnose an unexplained live failure (use take-a-step-back).
---

# Code Critique

Review returned implementation in two ordered layers. Conformance protects the accepted intent and proof boundary; adversarial critique looks for defects the brief and its named tests did not anticipate. Never certify from the delegate's return alone.

## Roles

Use a critic who did not write the implementation. Have the critic report findings only, and have the orchestrator adjudicate every finding and retain certification. Return accepted corrections to the implementer that produced the change, in its existing context when resumable. Never let the critic edit the implementation or dispose its own findings.

## The Materiality Criterion

Apply this test to every finding and give it to the critic verbatim:

> Would the change ship a defect, violate its brief, or damage what certifies it?

- **Material: report, adjudicate, and keep the review open.** Examples include wrong behavior, an omitted acceptance criterion, an unrelated change, a failure-path defect, a weakened test or gate, false verification evidence, or a change to trusted state the delegate does not own.
- **Not material: record, do not action, and never let it block.** Examples include taste, optional cleanup, naming preferences, and improvements that neither alter the brief nor affect behavior or proof.

Severity and materiality are separate. Require the verdict count to include only findings whose `material` value is true.

## Layer 1: Conformance

Review the accepted brief and the computed base-to-working-tree diff before reasoning about code quality.

For a dispatched implementation, run `metasystem validate conformance --stage review --job <job-id>`. It computes the diff from the implementer branch's merge-base with the invoking target checkout, includes committed, uncommitted, and previously untracked unignored work, persists `diff.patch` and `review.json`, rejects delegate changes under `plans/` or the agent control plane, and checks only that every changed path appears in the union of every round's immutable `diffBoundary` declarations: a declared-but-unchanged path passes, while a changed-but-undeclared path refuses. Carry the emitted `reviewedTree` into every code-critic return. Read that computed diff; never substitute the delegate's file list or summary.

Check that the diff implements every acceptance criterion, stays within the declared workspace and non-goals, preserves tests and certification assets unless the brief explicitly changes them, and contains no unrelated work. Treat every mismatch as a conformance finding before proceeding.

## Layer 2: Adversarial Critique

After conformance, attack the implementation itself. Trace changed control flow and state boundaries, then probe edge cases, error paths, lifecycle transitions, concurrency, input handling, and rollback or cleanup behavior in proportion to risk. Challenge whether the named tests prove the claim and run focused checks when they can distinguish a suspected defect from a hypothetical one.

Do not limit this layer to omissions in the brief. A conforming implementation can still be wrong. Report only evidence-backed findings in the shared format; do not rewrite the change.

## Findings and Dispositions

Use the shared critic `findings` array in canonical `return.json`, with stable `id`, `severity`, boolean `material`, `claim`, and `evidence` fields. Its human projection is:

```markdown
| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | high | yes | ... | ... |
```

The orchestrator answers every finding with the shared dispositions table:

```markdown
| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F-1 | accepted | ... | <where the implementation or proof changed> |
| F-2 | refuted | <the exact check and observed result> | none |
```

Use `accepted` or `refuted` for material findings; a TRUE finding outside the brief's declared threat model closes as `out-of-scope`, citing that scope in its evidence cell — accepted as fact, rejected as work. Use `noted` only for non-material findings. A chain closes on ZERO unrefuted material findings, and a refutation carries the exact check and its observed result — an evidence-free refutation is refused by the closure check itself. Close the round by running `bin/metasystem validate critique-closed --findings <return.json> --dispositions <file>`; a count or prose claim is not closure.

## Round Budget and Exit

When a design chain exited through fixtures-as-arbiter (see the design-critique skill), this code critique is MANDATORY and the named fixture obligations are part of its findings surface: an unimplemented or failing named fixture is a material finding.

Part One stores the review-round member in the goal's tier box: zero rounds
for Tier 1, two for Tier 2, and three for Tier 3. Mechanical accounting is
PENDING in Part Two under design point STR2-ROUND-ACCOUNTING-05: dispatch will
freeze that boundary on the critic chain, count rounds spent against it, and
make exhaustion open no fresh budget. Start every chain from
`scripts/agents/templates/review-brief.md` — round budget, threat model,
appetite, and scope declared BEFORE round one; a true finding outside the
declared threat model closes as out-of-scope citing the brief. Record the
goal's budget in the brief before review:

1. Run both layers over the full implementation and adjudicate every finding.
2. If corrections were required, send one focused follow-up to the same implementer, then recompute the whole diff and run both layers again.
3. While the reviewer's declared budget has a round left, review each focused follow-up and the whole recomputed diff.

As the reviewer's R-60-m1 rule, stop at the first round with zero material findings.

Only a finding that changes what gets built can keep the chain open and,
PENDING Part Two from design revision 3, it must name the artifact it would
change; that pending machinery demotes a finding that fails the artifact test.
If material findings remain, do not certify the change. Only an approved token
raising the goal's five-member tuple can raise its stored review-round member,
never above the three-round ceiling. PENDING Part Two from design revision 3, `job
critique-close` sends exhausted bounded findings to review obligations and
closes after accepted risk; until the accounting and close exist, stop with the
work waiting on the human. Never dispatch a silent fourth round.
