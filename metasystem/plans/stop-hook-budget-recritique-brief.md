Working Mode: implement
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-hook-budget-is-ours)
Date: 2026-09-06

# Review brief: the Stop hook budget, slice 1, closing re-review (chain shbo-build1-20260906, round 3)

FINDING IDS: chain-unique, continue the sequence: SHB-08, SHB-09, ... never F-n.

Round budget: this is the second and last review round of the chain.
Material only if it changes what gets built and names the artifact.

Round 1 (job shbo-cc1-20260906) found one material defect and four
accepted improvements; the dispositions are in
metasystem/plans/dispositions/stop-hook-budget-code-critique-r1.md and the
round-3 brief the builder worked from is
metasystem/plans/stop-hook-budget-fold2-brief.md. Round 3 is the tree
under review. The reviewer's out-of-scope finding (the parent trusts ps)
is goal stop-deadline-parent-trusts-ps and is not re-litigated here.

Threat model, unchanged from round 1 (metasystem/plans/stop-hook-budget-code-critique-brief.md),
plus the fold's own risks: the exit-2 retry masking a real failure (a
bare retry after any non-2 exit, or a retry that changes the completion's
result or outcome); the retry helper dropping an argument one of the
four call sites passed; the same-key retry clearing more than the
elapsed field; the deadline assertion's range wrong for the numbers; the
comment sweep touching code.

Scope: the computed diff of the implementer job under review.
Contract: metasystem/plans/stop-hook-budget-build-brief.md,
metasystem/plans/stop-hook-budget-fix-brief.md and
metasystem/plans/stop-hook-budget-fold2-brief.md, and the goal record
metasystem/plans/goals/stop-hook-budget-is-ours.md.

# Mandate

1. Each of the five dispositions (SHB-01, 03, 05, 06, 07) is implemented
   as written, or you name the one that is not and the artifact.
2. The retry fires only on exit 2 of a flagged call and repeats the same
   completion without the flag; every other exit is handled as before.
3. Nothing from round 2 regressed: the numbers, the measurement on every
   emission path, the optional flag, the pointer semantics.
4. Nothing outside the named owners changed (the two comment sites in
   metasystem/internal/goal/project.go are named owners for comment text
   only).

If nothing material remains, say so; that closes the chain.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
shbo-build1-20260906-r3.

# Gap Rule

stop and report a gap; never fill it silently.
