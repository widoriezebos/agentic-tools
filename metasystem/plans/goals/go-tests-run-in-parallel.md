# go-tests-run-in-parallel

- State: approved
- Priority: 1
- Sequence: 48
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="Test-only and gate-only changes; a wrong parallelisation shows as a race under -race or a red, a wrong ratchet shows as a refused gate; nothing reaches production behaviour"
- Tier: 2
- Intent: Go tests run in parallel and stay parallel by machinery. DONE when internal/goal runs with t.Parallel on every test and independent subtest with no shared process state, the audit parallel-ratchet verb refuses any package whose serial-test count rises above its recorded floor from the fast gate, and the floor only ratchets down.
- Origin: human
- Next step: Opened 09-18 by Wido (YES, ASAP; machinery). U1 internal/goal tests parallel (dm-gpar) and U2 audit parallel-ratchet guard in the fast gate (dm-tguard) in flight on a2d38afad38379fcd9a4747f18adbe3035958309. Next: land both in wave 3e, then a systematic pass over the other serial packages under the ratchet.
- OpenedAt: 2026-09-18T14:56:01Z
- Revision: 4
- Budget: elapsedLimit=1d attemptLimit=20 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-18T14:59:25Z revision=3 opid=XRMD7ZBWBBG03DHS0YFF8E1BH5-m1e-c6925449 authority=proven digest=97d790857a9e1bfceaec9d311599579d3af851a1ebc6594c70d3243d04da4d31

History:
- 2026-09-18T14:56:01Z P620H84YGCSWX7ATHC0Z2PN5ZK-m1e-c6925449 open actor=human:Wido targets=go-tests-run-in-parallel
- 2026-09-18T14:58:31Z 9WAFAEVQBWZ67FK2Y67ZN3HHH5-m1e-c6925449 edit actor=human:Wido targets=go-tests-run-in-parallel
- 2026-09-18T14:59:25Z XRMD7ZBWBBG03DHS0YFF8E1BH5-m1e-c6925449 approve actor=human:Wido targets=go-tests-run-in-parallel
- 2026-09-18T15:00:50Z NP8YFV8F0VVX319C87YZTA09T5-m1e-c6925449 set-priority actor=human:Wido targets=go-tests-run-in-parallel reason=priority-order subject=go-tests-run-in-parallel from=unranked to=1:48 requested-sequence=48
Integrity: sha256=faac9f41f8b2b3b2d617a0c35b6a041b18c7a185b9d8ca410bb1dc50c8c86b26
