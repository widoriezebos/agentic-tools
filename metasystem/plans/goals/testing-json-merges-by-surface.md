# testing-json-merges-by-surface

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="A merge helper for one file; a wrong merge is caught by the contract tests at the gate."
- Tier: 2
- Intent: testing.json merges without a hand rebase. DONE: (1) two branches that each add surfaces or groups merge by surface name, not by line, with the residual lists recomputed; (2) the merge lane and the branch verbs use it; (3) no unit lands with a hand-rebased testing.json hunk after this lands.
- Origin: human
- Next step: Evidence 2026-09-17: every batch merge today needed rebase-testing-json3.py from a seat scratchpad because the contract's arrays conflict on lines, and the three-way merge has no owner. No design first: brief the surface-keyed union, build, read. Lands beside the branch verbs of goals-live-on-branches.
- OpenedAt: 2026-09-17T13:39:42Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-17T13:39:42Z V6KR2GQBNYEPY0MZZ2JBCFTFXJ-m1e-c6925449 open actor=human:Wido targets=testing-json-merges-by-surface
Integrity: sha256=6b514ac90919194de122e082d4289c5259c52667b5c1aebb6bcd15f8e9d7e80c
