# the-fleet-serialization-proof-asserts-cleared-leases

- State: queued
- Risk: severity=2 novelty=1 exposure=1 accumulation=1 basis="a test-bed repair of one test from a read root cause (Explore 2026-11-07 16:1x); no production change; the unit check and the lane proof gate it"
- Tier: 2
- Intent: TestFleetHostViewPublicMachineList asserts exactly 1 host lease, but the engine closes the slot before the marker (internal/proofrun/host_resources.go:160-171, :1030, :1052), so the second proof can skip a still-locked cleared marker (:487-509) and leave 2 cleared markers; the test asserts every lease cleared (assertHostAdmissionClean(t, admission, 0) at cmd/metasystem/fleet_host_view_test.go:249) and Close releases the marker before the slot
- Origin: human
- Next step: Build without a design from the Explore evidence of 2026-11-07: the assertion change and the Close order; about 10 lines
- OpenedAt: 2026-11-07T15:56:58Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T15:56:58Z K9Q0BFPX89DHE68QTFRX18PNGD-m1e-9f79adc8 open actor=human:Wido targets=the-fleet-serialization-proof-asserts-cleared-leases
Integrity: sha256=944d17cf4f60ea6245d34ef617c129ce5320917ac223ff4bdd7b01d70dec4866
