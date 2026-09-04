Working Mode: implement
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal finding-register-id-collision-across-chains)
Date: 2026-09-04

# Review brief: the finding-register scope fix (chain frc-build1)

FINDING IDS: chain-unique, FRC-01, ... never F-n.

Round budget: 1 focused round, then at most one correction and its
re-review (tier 2). R-60-m1's rule: material only if it changes what
gets built and names the artifact.

Threat model: the lawful union weakened (two critic roots on the SAME
subject with conflicting classes must still refuse; the critic-shopping
hole of design point STR2-CRITIC-UNION-11 must stay closed); the
subject resolution reading the wrong field, so roots of one subject
look different or roots of different subjects look the same; a
register state from older records without a reviews reference
crashing or silently passing. Out: the design's future same-tree union
in conformance (part two of the tiering machinery); taste.

Scope: the computed diff of the implementer job under review.
Contract: metasystem/plans/finding-register-id-collision-across-chains-build-brief.md
and the goal record.

# Mandate

1. Different subjects never conflict; the same subject still refuses;
   a re-issued round advances; the three fixtures exist and pass.
2. Older records without a subject resolve deterministically and
   safely (say which way, and why it is fail-closed enough).
3. The conflict message names both subjects and the remedy.

If nothing material remains, say so; that closes the chain and the fix
lands.

# Constraints

Wall-clock budget: 25 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
frc-build1.

# Gap Rule

stop and report a gap; never fill it silently.
