# every-detached-test-helper-is-in-custody

- State: parked
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="Test code only: detached test helpers move into the fixture custody that exists; production code is unchanged; a fault is a test that fails at once"
- Tier: 1
- Intent: Every test that starts a detached helper (a SysProcAttr with Setsid or Setpgid) starts it in the existing fixture custody, so the detached-helper audit's allowance of a-test-helper-dies-with-its-test is empty (split from that goal, V6 of its U1 brief; R-148-m1e)
- Origin: main
- Next step: After a-test-helper-dies-with-its-test lands: take the allowance of TestAuditNoTestStartsADetachedHelperOutsideCustody (cmd/metasystem/audit_detached_helper_test.go) package by package, internal/up's owner helpers and ownerfixture.Component's callers without a fixture word first; each package's files leave the allowance in the same unit; no age-based sweep
- OpenedAt: 2026-11-03T16:10:17Z
- Revision: 2
- BlockedBy: a-test-helper-dies-with-its-test
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-03T16:10:27Z revision=2 opid=25FKMDCM3F7HMSW24F7YM1DVRK-m1e-7189b09c authority=proven digest=a0dbb7b68062aeee4c264155cba1fe5373a5ae0c27d9cae42a365957b7c79aa8 episode=2
- Parked: by=human:Wido at=2026-11-03T16:10:17Z blocker=a-test-helper-dies-with-its-test because=blocked by a-test-helper-dies-with-its-test; returns when it is done

History:
- 2026-11-03T16:10:17Z 7WCB7NNECGNMVDJHCV862ZCQPT-m1e-7189b09c open actor=human:Wido targets=every-detached-test-helper-is-in-custody,a-test-helper-dies-with-its-test
- 2026-11-03T16:10:27Z 25FKMDCM3F7HMSW24F7YM1DVRK-m1e-7189b09c approve actor=human:Wido targets=every-detached-test-helper-is-in-custody
Integrity: sha256=f507b00c72859468601e6ffde3bd31c9dbb3aa9ee4388ec56fe88f6848397605
