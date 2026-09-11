# command-package-tests-red-inside-the-race-gate

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: the cadence battery's Go engine gate is red on tests no landing sees, so the battery cannot be green; novelty 1: the tests pass outside the gate and the difference is the gate's own process ancestry; exposure 3: every cadence run on every seat; accumulation 1: none"
- Tier: 3
- Intent: The race gate's cmd tests are green inside the gate. On 2026-09-11 the gate's cmd run got its output kept (baa3a70e) and showed TestRecertifiedLandingParksOnOriginMove (unexpected park record: chain-recertification-test-command-refused), TestRecertifiedLandingPublishesNestedSchemaTwoCandidate (nested schema-two landing failed: code=1) and TestProofRunCommandGovernedParentSharesOneCharge failing only when go test -race ./cmd/... runs inside section/go-engine-gate; all three pass under -race from a plain shell, in an extracted snapshot layout, and with the gate's environment variables set (m1e evidence, attempts proof-mtx87lgk, proof-mtx8zmcn, proof-mtx9zmcn era), so the difference is the process ancestry the gate runs under (a proof attempt's section worker), not the environment. Nobody saw these reds while the gate discarded its cmd output. DONE means the three tests pass inside section/go-engine-gate in a cadence run, by making the tests independent of their caller's ancestry or by naming the ancestry law they hit, and the gate stays green for three cadence runs.
- Origin: main
- Next step: Reproduce under the gate's ancestry (run go test -race ./cmd/... as a section worker child through test run --groups section/go-engine-gate --purpose diagnostic, or attach the provenance the refusal names), then fix the tests' assumptions.
- OpenedAt: 2026-09-11T18:25:53Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-11T18:25:53Z 5ZEHYWTQQ7EK2M4ATTXNGYVXKV-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=command-package-tests-red-inside-the-race-gate
Integrity: sha256=1892bc930f385e5f2b22544d9b143102425a5c3249c44f9b203cba67d7c67732
