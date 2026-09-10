Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-10

# Review brief: the obligation-state package's owning test surface (chain dispatch-cap-surface1-20260910)

FINDING IDS: chain-unique, DCS-01, DCS-02, ... never F-n. Report `round`
as 1 in your return: it is this job's own round. One focused round.

Why: since 1b12f534 the testing contract metasystem/testing.json
declared no surface for metasystem/internal/obligationstate/**, so the
landing of goal dispatch-cap-necessity (whose reviewed chain touches
internal/obligationstate/state_test.go) could not resolve delivery
impact. The implementer added that path to the dispatch-goal-mission
surface and an obligationstate-standard unit group shaped like
goal-decision-standard. A tier-1 landing was refused for a tier-3 goal,
so this one-file change lands as a reviewed chain: you are its review.
The seat's deep receipt on the candidate passed every selected group,
obligationstate-standard included.

Scope: the computed diff of job dispatch-cap-surface1-20260910 (one
file, metasystem/testing.json). Contract:
metasystem/plans/application-testing-contract-design.md (rule 6:
bounded policy correction).

# Mandate

1. The change is exactly the surface path and the group, shaped like
   the neighbouring group, with nothing else in the contract moved.
2. The group runs the package's tests (go, cwd metasystem, package
   internal/obligationstate) with the same tools and obligations shape.
3. Nothing outside metasystem/testing.json changed.

If nothing material remains, say so; that closes the chain and it
lands.

# Constraints

Wall-clock budget: 10 minutes. Return per the code-critic schema with
the reviewedTree from the review record beside the computed diff (the
conformance validator refuses from a sandbox; say so).

# Gap Rule

Stop adding at a gap that needs a decision no page has made; keep what
is written; report the gap with the resolution you propose. Never
delete written work.
