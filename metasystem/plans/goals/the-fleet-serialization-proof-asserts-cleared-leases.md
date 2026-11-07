# the-fleet-serialization-proof-asserts-cleared-leases

- State: approved
- Risk: severity=2 novelty=1 exposure=1 accumulation=1 basis="a test-bed repair of one test from a read root cause (Explore 2026-11-07 16:1x); no production change; the unit check and the lane proof gate it"
- Tier: 2
- Intent: TestFleetHostViewPublicMachineList asserts exactly 1 host lease, but the engine closes the slot before the marker (internal/proofrun/host_resources.go:160-171, :1030, :1052), so the second proof can skip a still-locked cleared marker (:487-509) and leave 2 cleared markers; the test asserts every lease cleared (assertHostAdmissionClean(t, admission, 0) at cmd/metasystem/fleet_host_view_test.go:249) and Close releases the marker before the slot
- Origin: human
- DesignGate: off
- Next step: Build without a design from the Explore evidence of 2026-11-07: the assertion change and the Close order; about 10 lines
- OpenedAt: 2026-11-07T15:56:58Z
- Revision: 4
- Pinned: m1f
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-07T15:57:20Z revision=3 opid=FKXPS80SP5VG9W8VW5JSXXYJSY-m1e-9f79adc8 authority=proven digest=4190599beed073bba7cf4974013c2806ac419fbdb694b93ab2b09e9b4d5b59a8 episode=3

History:
- 2026-11-07T15:56:58Z K9Q0BFPX89DHE68QTFRX18PNGD-m1e-9f79adc8 open actor=human:Wido targets=the-fleet-serialization-proof-asserts-cleared-leases
- 2026-11-07T15:57:09Z EAFRTHGMQN9FBWB45KKGYXMG8M-m1e-9f79adc8 edit actor=human:Wido targets=the-fleet-serialization-proof-asserts-cleared-leases reason=Allowed: build-without-design why=Allowed: build-without-design. A one-unit repair from a read root cause impact=This lets goal the-fleet-serialization-proof-asserts-cleared-leases be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of the-fleet-serialization-proof-asserts-cleared-leases until withdrawn. To undo: metasystem goal disallow the-fleet-serialization-proof-asserts-cleared-leases build-without-design
- 2026-11-07T15:57:20Z FKXPS80SP5VG9W8VW5JSXXYJSY-m1e-9f79adc8 approve actor=human:Wido targets=the-fleet-serialization-proof-asserts-cleared-leases
- 2026-11-07T15:57:30Z T9S9NJ3YVBJHRQ24MPF3Y0VKNT-m1e-9f79adc8 set-pin actor=human:Wido targets=the-fleet-serialization-proof-asserts-cleared-leases
Integrity: sha256=bd2ea8e40575149efb804b036b5a84c6305372921bffa03939ae3491f0a1d54e
