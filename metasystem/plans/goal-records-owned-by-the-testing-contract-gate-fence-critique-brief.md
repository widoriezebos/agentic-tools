Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal goal-records-owned-by-the-testing-contract)
Date: 2026-09-10

# Review brief: the gate-fence section moves to the cadence set (chain gate-fence-cadence1-20260910)

FINDING IDS: chain-unique, GFC-01, GFC-02, ... never F-n. Report `round`
as 1 in your return: it is this job's own round. One focused round.

Why: since 1b12f534 the landing receipt copies the enrolled engine into
the isolated candidate worktree, and section/gate-fence-fixtures runs
the dispatch skew preflight there, which refuses any candidate that
changes engine code; a candidate-built engine cannot be enrolled. Three
reviewed chains are stuck. Wido approved on 2026-09-10 moving the
section out of the runtime-custody surface's deep list while it stays
in the cadence set, until the receipt runs its sections with a proof
engine built from the candidate (m1c's work).

Scope: the computed diff of job gate-fence-cadence1-20260910
(metasystem/testing.json only). Contract:
metasystem/plans/application-testing-contract-design.md (rule 6).

# Mandate

1. Exactly one list entry removed (runtime-custody deep:
   section/gate-fence-fixtures); the cadence set still names it;
   nothing else moved.
2. The contract validates; the testpolicy tests pass.

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
