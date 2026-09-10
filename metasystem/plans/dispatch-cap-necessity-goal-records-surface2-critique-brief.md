Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-10

# Review brief: goal records get an owning test surface (chain dispatch-cap-surface3-20260910)

FINDING IDS: chain-unique, DCS-07, DCS-08, ... never F-n. Report `round`
as 1 in your return: it is this job's own round. One focused round.

Why: since 1b12f534 the testing contract owned no metasystem/plans/**,
metasystem/records/** or memory/rulings.md, so no seat could land a
goal's briefs and dispositions ("no surface owns changed path"). The
implementer added a goal-records surface with the instructions
surface's static groups; the selection test of the previous chain
(reviewed clean, unlandable: a Go file trips the receipt's skew
preflight) waits for a later chain. The same bounded policy
correction landed for internal/obligationstate as c0d5b136.

Scope: the computed diff of job dispatch-cap-surface3-20260910
(metasystem/testing.json only). Contract:
metasystem/plans/application-testing-contract-design.md (rule 6).

# Mandate

1. The surface owns exactly the four path patterns, with the
   instructions surface's groups and dependency, and no other surface
   lost a path; overlap with instructions, if any, is what the
   validator allows.
2. The contract validates; the existing testpolicy tests pass.
3. Nothing outside those files changed.

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
