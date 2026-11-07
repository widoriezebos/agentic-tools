# a-scratch-cleanup-test-drains-like-production

- State: approved
- Priority: 1
- Sequence: 126
- Risk: severity=2 novelty=1 exposure=1 accumulation=1 basis="a test-bed repair of one test from a read root cause (Explore 2026-11-07 16:1x); no production change; the unit check and the lane proof gate it"
- Tier: 2
- Intent: TestScratchCleanupDrainIsBoundedForALiveWriter fails on a fork copy of the writer lock: its second Cleanup uses the single-probe scratchDrain{} (internal/proofrun/scratch_test.go:384) while production retries for 2 s (defaultScratchDrain, scratch.go:160, :600); the test waits the copy out on an injected clock (R-104) the way production does; no production change
- Origin: human
- DesignGate: off
- Next step: Build without a design from the Explore evidence of 2026-11-07: replace the single probe with a bounded drain on the test clock; one test; about 15 lines
- OpenedAt: 2026-11-07T15:52:27Z
- Revision: 5
- Pinned: m1f
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-07T15:56:14Z revision=3 opid=Z07S4WF1RB3PJFHPJ21BJ5T0CK-m1e-9f79adc8 authority=proven digest=58e428087194f98968d531fa717ca05632d8aaef0eb2bea9fe1e8c60c41164d3 episode=3

History:
- 2026-11-07T15:52:27Z NE5FH3355R8VE4N5GK61J2RPCW-m1e-9f79adc8 open actor=human:Wido targets=a-scratch-cleanup-test-drains-like-production
- 2026-11-07T15:54:41Z TQNEZG5ZXS8G267K13B2QZEAHJ-m1e-9f79adc8 edit actor=human:Wido targets=a-scratch-cleanup-test-drains-like-production reason=Allowed: build-without-design why=Allowed: build-without-design. A one-unit test repair from a read root cause impact=This lets goal a-scratch-cleanup-test-drains-like-production be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of a-scratch-cleanup-test-drains-like-production until withdrawn. To undo: metasystem goal disallow a-scratch-cleanup-test-drains-like-production build-without-design
- 2026-11-07T15:56:14Z Z07S4WF1RB3PJFHPJ21BJ5T0CK-m1e-9f79adc8 approve actor=human:Wido targets=a-scratch-cleanup-test-drains-like-production
- 2026-11-07T15:56:27Z Q0RDTAB2QHG8Y2HEC9HM0HDVR5-m1e-9f79adc8 set-pin actor=human:Wido targets=a-scratch-cleanup-test-drains-like-production
- 2026-11-07T15:56:38Z F21CXR1FYYQE1ZSH072MBG6907-m1e-9f79adc8 set-priority actor=human:Wido targets=a-scratch-cleanup-test-drains-like-production reason=priority-order subject=a-scratch-cleanup-test-drains-like-production from=unranked to=1:126 requested-sequence=append
Integrity: sha256=76cb88a46d42e2a808843be5f377e5ebe8557f5612f1b0653db0e985419af40d
