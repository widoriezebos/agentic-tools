# a-fixture-launch-counts-only-real-competitors

- State: approved
- Risk: severity=3 novelty=1 exposure=3 accumulation=1 basis="a deadlock in the lane proof blocks every landing that runs cmd/metasystem whole; the fix touches the proof launcher census, which every proof launch reads; the test bed change is local"
- Tier: 3
- Intent: Main red: TestLegacyProofLaunchRealLauncherRequiresLiveReporter deadlocks inside every lane proof that runs cmd/metasystem whole (finding 553). Its nested legacy launch counts real proof launchers on the host (hostLauncherCensus via countProofLaunchers, internal/proofrun/attemptload.go:343) while its managed-marker filter (managedProofProcess, host_resources.go:1167-1178) reads the test admission folder, so every real launcher counts as a competitor and the child waits forever (no per-test deadline). Fix the census to read managed markers where real launchers write them, set the test bed admission cap to 0 at proof_run_test.go:3722, and pin it with TestLegacyProofLaunchIgnoresAnUnrelatedLauncher
- Origin: human
- DesignGate: off
- Next step: Build without a design from finding 553 (root cause in its note): one unit; the engine census fix, the one-line test bed fix, the decoy-launcher test; brief by the seat
- OpenedAt: 2026-11-07T05:03:39Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-07T05:06:17Z revision=3 opid=XBQNDB23H4ASN9TABEV1KP8KQS-m1e-88b0c4a5 authority=proven digest=49542e24bc0269568b17d7b8f9b76e4bc3aad2aece5487d4fedc287ee52f2e2a episode=3

History:
- 2026-11-07T05:03:39Z CPAP6KKN374N8C80RS90EPZX52-m1e-88b0c4a5 open actor=human:Wido targets=a-fixture-launch-counts-only-real-competitors
- 2026-11-07T05:04:47Z ZQ1Z5QTNJ50A4QNKH0PK9ADV6V-m1e-88b0c4a5 edit actor=human:Wido targets=a-fixture-launch-counts-only-real-competitors reason=Allowed: build-without-design why=Allowed: build-without-design. A main-red fix of one unit from a recorded finding (553) whose root cause is read; the lane proof and the read gate it impact=This lets goal a-fixture-launch-counts-only-real-competitors be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of a-fixture-launch-counts-only-real-competitors until withdrawn. To undo: metasystem goal disallow a-fixture-launch-counts-only-real-competitors build-without-design
- 2026-11-07T05:06:17Z XBQNDB23H4ASN9TABEV1KP8KQS-m1e-88b0c4a5 approve actor=human:Wido targets=a-fixture-launch-counts-only-real-competitors
Integrity: sha256=9f78efc4592c16375bc9ff71fb37dbdf3a996a71cd1ca4d1f08f6decb94a465f
