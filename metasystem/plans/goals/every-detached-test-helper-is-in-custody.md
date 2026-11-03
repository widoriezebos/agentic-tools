# every-detached-test-helper-is-in-custody

- State: parked
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="Test code only: detached test helpers move into the fixture custody that exists; production code is unchanged; a fault is a test that fails at once"
- Tier: 1
- Intent: Every test that starts a detached helper (a SysProcAttr with Setsid or Setpgid) starts it in the existing fixture custody, so the detached-helper audit's allowance of a-test-helper-dies-with-its-test is empty (split from that goal, V6 of its U1 brief; R-148-m1e)
- Origin: main
- Next step: After a-test-helper-dies-with-its-test lands: take the allowance of TestAuditNoTestStartsADetachedHelperOutsideCustody (cmd/metasystem/audit_detached_helper_test.go) package by package, internal/up's owner helpers and ownerfixture.Component's callers without a fixture word first; each package's files leave the allowance in the same unit; no age-based sweep
- OpenedAt: 2026-11-03T16:10:17Z
- Revision: 1
- BlockedBy: a-test-helper-dies-with-its-test
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Parked: by=human:Wido at=2026-11-03T16:10:17Z blocker=a-test-helper-dies-with-its-test because=blocked by a-test-helper-dies-with-its-test; returns when it is done

History:
- 2026-11-03T16:10:17Z 7WCB7NNECGNMVDJHCV862ZCQPT-m1e-7189b09c open actor=human:Wido targets=every-detached-test-helper-is-in-custody,a-test-helper-dies-with-its-test
Integrity: sha256=c6a6a3ff98b0827a6d7ec6a4957a3640146a918717ead3a6fd73b1e3292b6059
