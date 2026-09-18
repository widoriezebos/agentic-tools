# go-tests-run-in-parallel

- State: approved
- Priority: 1
- Sequence: 48
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="Test-only and gate-only changes; a wrong parallelisation shows as a race under -race or a red, a wrong ratchet shows as a refused gate; nothing reaches production behaviour"
- Tier: 2
- Intent: Go tests run in parallel and stay parallel by machinery. DONE when internal/goal runs with t.Parallel on every test and independent subtest with no shared process state, the audit parallel-ratchet verb refuses any package whose serial-test count rises above its recorded floor from the fast gate, and the floor only ratchets down.
- Origin: human
- Next step: U1 and U2 on main d2713932efcdc9a3adee0493bd8196f3996e1bb4; U3 = internal/goal t.Run subtests under t.Parallel, plus a clock seam for the attention_test and txn_test waits; then conclude.
- OpenedAt: 2026-09-18T14:56:01Z
- Revision: 5
- Budget: elapsedLimit=1d attemptLimit=20 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-18T14:59:25Z revision=3 opid=XRMD7ZBWBBG03DHS0YFF8E1BH5-m1e-c6925449 authority=proven digest=97d790857a9e1bfceaec9d311599579d3af851a1ebc6594c70d3243d04da4d31

History:
- 2026-09-18T14:56:01Z P620H84YGCSWX7ATHC0Z2PN5ZK-m1e-c6925449 open actor=human:Wido targets=go-tests-run-in-parallel
- 2026-09-18T14:58:31Z 9WAFAEVQBWZ67FK2Y67ZN3HHH5-m1e-c6925449 edit actor=human:Wido targets=go-tests-run-in-parallel
- 2026-09-18T14:59:25Z XRMD7ZBWBBG03DHS0YFF8E1BH5-m1e-c6925449 approve actor=human:Wido targets=go-tests-run-in-parallel
- 2026-09-18T15:00:50Z NP8YFV8F0VVX319C87YZTA09T5-m1e-c6925449 set-priority actor=human:Wido targets=go-tests-run-in-parallel reason=priority-order subject=go-tests-run-in-parallel from=unranked to=1:48 requested-sequence=48
- 2026-09-18T20:05:13Z W6GCB00T87FX3E8D93HDR8KQ52-m1e-c6925449 edit actor=human:Wido targets=go-tests-run-in-parallel
Integrity: sha256=31abb757a231a4ed4e2c07c09b67f3d654f0324e3d597a642baa40af05af6fd4
