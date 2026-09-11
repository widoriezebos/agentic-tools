# deep-battery-under-ten-minutes

- State: approved
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: a wrong merge of shard results could pass a red test unseen; novelty 2: a worker pool and shard merging are new shapes in proofrun; exposure 3: every cadence run and every seat consumes the result; accumulation 2: a flake introduced by concurrency compounds across landings"
- Tier: 3
- Intent: The cadence run (test run --mode deep --purpose cadence) completes in under ten minutes wall on the 18-core Mac with every cadence group still present and passing, and no slower than today on the 4-vCPU VM. DONE is that sentence, with three consecutive green cadence runs and their attempt ids in the receipt. Slices 2, 3 and 5 of plans/suite-speed-plan.md, arc-sized: groups run concurrently inside one attempt under a configurable cap; the long Go groups are sharded and the big fixture sections split into per-scenario groups; the race gate reuses the shards.
- Origin: human
- Next step: INTENT: wall time of the battery equals its longest group, then no group over about three minutes. CONSTRAINTS (section 7 of the plan): no early stop, evidence preserved per group exactly as today, no scheduler or daemon or parallel proof store (the pool lives inside the single worker process), decisions in Go and plumbing in Bash, the VM never slower than today with its own measured cap, no group deleted from testing.json. FREEDOMS: the cap key and defaults (Wido decides for the VM), the shard field's shape, the section ids of the split. Before design: the spike of section 10 step 3 (two heavy sections side by side by hand, shared paths and leaked children recorded) and the coverage-merge spike of step 4. Split into slices of about four hours each (goal split before claiming); each slice: Fable designs, Sol critiques, Sol implements, Fable critiques, land.
- OpenedAt: 2026-09-10T12:02:35Z
- Revision: 3
- Pinned: m1e
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T14:32:42Z revision=2 opid=N1VRJEJY0P48662SMEDYE5087Q-m1-f47a9d40 authority=proven digest=b2e04a693d659d401bbd6fa3596b1ddbf0d3ff57f7cd77c5abc38109d32c44c1

History:
- 2026-09-10T12:02:35Z T349JE65PA2AZGXK28VA4PN6T1-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=deep-battery-under-ten-minutes
- 2026-09-11T14:32:42Z N1VRJEJY0P48662SMEDYE5087Q-m1-f47a9d40 approve actor=human:Wido targets=deep-battery-under-ten-minutes
- 2026-09-11T15:47:54Z Z6E84QPYZT7QGXBM7406J3G6ZF-m1-c6925449 set-pin actor=human:Wido targets=deep-battery-under-ten-minutes
Integrity: sha256=8b654a4c295e6e60b8f7b5bcdf90222e76033778ba979feae32199c2c8c27f51
