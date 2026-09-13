# command-package-tests-red-inside-the-race-gate

- State: done
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: the cadence battery's Go engine gate is red on tests no landing sees, so the battery cannot be green; novelty 1: the tests pass outside the gate and the difference is the gate's own process ancestry; exposure 3: every cadence run on every seat; accumulation 1: none"
- Tier: 3
- Intent: The race gate's cmd tests are green inside the gate. On 2026-09-11 the gate's cmd run got its output kept (baa3a70e) and showed TestRecertifiedLandingParksOnOriginMove (unexpected park record: chain-recertification-test-command-refused), TestRecertifiedLandingPublishesNestedSchemaTwoCandidate (nested schema-two landing failed: code=1) and TestProofRunCommandGovernedParentSharesOneCharge failing only when go test -race ./cmd/... runs inside section/go-engine-gate; all three pass under -race from a plain shell, in an extracted snapshot layout, and with the gate's environment variables set (m1e evidence, attempts proof-mtx87lgk, proof-mtx8zmcn, proof-mtx9zmcn era), so the difference is the process ancestry the gate runs under (a proof attempt's section worker), not the environment. Nobody saw these reds while the gate discarded its cmd output. DONE means the three tests pass inside section/go-engine-gate in a cadence run, by making the tests independent of their caller's ancestry or by naming the ancestry law they hit, and the gate stays green for three cadence runs.
- Origin: main
- Next step: Cause found and fixed in bb1e2dfd, and it was never process ancestry: go-gate.sh exports GOFLAGS=-mod=readonly to every test it runs; the recertified-landing fixture proved through the receipt canary environment (no GOFLAGS) and landed through the scrubbed full environment (GOFLAGS present), so the toolchain digest in the group execution identity differed on the landing side only and the retained proof read as missing. Reproduced from a plain shell with that one variable. TestProofRunCommandGovernedParentSharesOneCharge had a second cause: the command package's TestMain scrub (b13771d7) unset METASYSTEM_PROOF_RUN_ROOT and _RUN_ID in the re-executed child before the engine read them. The fixture now lands in the receipt's environment and the governed locators travel under fixture names. Proof so far: go test -race ./cmd/... green under the gate's environment from a plain shell (236 s). DONE when the gate's cmd stage is green in three cadence runs (first: attempt proof-mtxe9exo in flight); conclude then.
- Concluded: Deprecated: DONE is met: the cause was fixed at bb1e2dfd (GOFLAGS in the group execution identity plus the command package TestMain scrub) and section/go-engine-gate is passed with purpose cadence in proof-mty4qhaz-af176dcea378dae3, proof-mty6o5n7-c165976081332c07 and proof-mty9ejoz-9c626c90b9464750 (artifacts/. Concluded 2026-09-13 in the backlog consolidation on Wido's word.
- OpenedAt: 2026-09-11T18:25:53Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-11T18:25:53Z 5ZEHYWTQQ7EK2M4ATTXNGYVXKV-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=command-package-tests-red-inside-the-race-gate
- 2026-09-11T20:19:27Z 9N3JCC5GQXYQ8EBWMC1SWWZ5N9-m1e-892cdaec edit actor=m1e+main-1789030447-51011-5722fc targets=command-package-tests-red-inside-the-race-gate
- 2026-09-13T08:22:52Z APQWRSNJA67YTF69136CZE7PJC-m1-c6925449 done actor=human:Wido targets=command-package-tests-red-inside-the-race-gate
Integrity: sha256=7abd3ce7afd8a839314a44d48547de014e047b3583ec10a05a2cea27945f7744
