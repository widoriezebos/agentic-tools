# testing-json-merges-by-surface

- State: approved
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="A merge helper for one file; a wrong merge is caught by the contract tests at the gate."
- Tier: 2
- Intent: testing.json merges without a hand rebase. DONE: (1) two branches that each add surfaces or groups merge by surface name, not by line, with the residual lists recomputed; (2) the merge lane and the branch verbs use it; (3) no unit lands with a hand-rebased testing.json hunk after this lands.
- Origin: human
- Next step: Evidence 2026-09-17: every batch merge today needed rebase-testing-json3.py from a seat scratchpad because the contract's arrays conflict on lines, and the three-way merge has no owner. No design first: brief the surface-keyed union, build, read. Lands beside the branch verbs of goals-live-on-branches.
- OpenedAt: 2026-09-17T13:39:42Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-17T13:40:00Z revision=2 opid=3TGNQ0V47XSAP3XK7PTYRBC585-m1e-c6925449 authority=proven digest=f093d05b4355aa8885f19c12a1d69eaaf9a6ab9a7c9229c38154591773e87589

History:
- 2026-09-17T13:39:42Z V6KR2GQBNYEPY0MZZ2JBCFTFXJ-m1e-c6925449 open actor=human:Wido targets=testing-json-merges-by-surface
- 2026-09-17T13:40:00Z 3TGNQ0V47XSAP3XK7PTYRBC585-m1e-c6925449 approve actor=human:Wido targets=testing-json-merges-by-surface
Integrity: sha256=b8339a4e0d0bc4df5e5ff29a1aeaf6a1d3426af88aee02fcb20630f8d0b72765
