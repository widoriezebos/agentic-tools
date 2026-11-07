# a-scratch-cleanup-test-drains-like-production

- State: queued
- Risk: severity=2 novelty=1 exposure=1 accumulation=1 basis="a test-bed repair of one test from a read root cause (Explore 2026-11-07 16:1x); no production change; the unit check and the lane proof gate it"
- Tier: 2
- Intent: TestScratchCleanupDrainIsBoundedForALiveWriter fails on a fork copy of the writer lock: its second Cleanup uses the single-probe scratchDrain{} (internal/proofrun/scratch_test.go:384) while production retries for 2 s (defaultScratchDrain, scratch.go:160, :600); the test waits the copy out on an injected clock (R-104) the way production does; no production change
- Origin: human
- Next step: Build without a design from the Explore evidence of 2026-11-07: replace the single probe with a bounded drain on the test clock; one test; about 15 lines
- OpenedAt: 2026-11-07T15:52:27Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T15:52:27Z NE5FH3355R8VE4N5GK61J2RPCW-m1e-9f79adc8 open actor=human:Wido targets=a-scratch-cleanup-test-drains-like-production
Integrity: sha256=3e55c68d8d66d5c2cb21cefbe5d3943f72ccdd056188ef5ea63531441b0eb514
