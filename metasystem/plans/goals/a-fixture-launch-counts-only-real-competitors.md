# a-fixture-launch-counts-only-real-competitors

- State: queued
- Risk: severity=3 novelty=1 exposure=3 accumulation=1 basis="a deadlock in the lane proof blocks every landing that runs cmd/metasystem whole; the fix touches the proof launcher census, which every proof launch reads; the test bed change is local"
- Tier: 3
- Intent: Main red: TestLegacyProofLaunchRealLauncherRequiresLiveReporter deadlocks inside every lane proof that runs cmd/metasystem whole (finding 553). Its nested legacy launch counts real proof launchers on the host (hostLauncherCensus via countProofLaunchers, internal/proofrun/attemptload.go:343) while its managed-marker filter (managedProofProcess, host_resources.go:1167-1178) reads the test admission folder, so every real launcher counts as a competitor and the child waits forever (no per-test deadline). Fix the census to read managed markers where real launchers write them, set the test bed admission cap to 0 at proof_run_test.go:3722, and pin it with TestLegacyProofLaunchIgnoresAnUnrelatedLauncher
- Origin: human
- Next step: Build without a design from finding 553 (root cause in its note): one unit; the engine census fix, the one-line test bed fix, the decoy-launcher test; brief by the seat
- OpenedAt: 2026-11-07T05:03:39Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T05:03:39Z CPAP6KKN374N8C80RS90EPZX52-m1e-88b0c4a5 open actor=human:Wido targets=a-fixture-launch-counts-only-real-competitors
Integrity: sha256=9ccf317da0434035369df70acb7014ea4c4002d6ae77fad7cd2dc38604398b4c
