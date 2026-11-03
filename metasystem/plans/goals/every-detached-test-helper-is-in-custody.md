# every-detached-test-helper-is-in-custody

- State: approved
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="Test code only: detached test helpers move into the fixture custody that exists; production code is unchanged; a fault is a test that fails at once"
- Tier: 1
- Intent: Every test that starts a detached helper (a SysProcAttr with Setsid or Setpgid) starts it in the existing fixture custody, so the detached-helper audit's allowance of a-test-helper-dies-with-its-test is empty (split from that goal, V6 of its U1 brief; R-148-m1e)
- Origin: main
- Next step: After a-test-helper-dies-with-its-test lands: take the allowance of TestAuditNoTestStartsADetachedHelperOutsideCustody (cmd/metasystem/audit_detached_helper_test.go) package by package, internal/up's owner helpers and ownerfixture.Component's callers without a fixture word first; each package's files leave the allowance in the same unit; no age-based sweep
Open read items (fix unit astra-brief-r1): 1
- ReadItem: id=astra-brief-r1-1 read=astra-brief-r1 state=open addedAt=2026-11-03T16:33:13Z changedAt=- material=true closingReference="" text="R-4 residue of U1 (brief V6, Astra brief round 1 finding 4): the detached-helper audit's allowance (about 40 test files, 52 detached launches outside fixture custody, internal/up's owner helpers, ownerfixture.Component callers without a fixture word) moves to follow-up goal every-detached-test-helper-is-in-custody, which m1e is asked to open (a seat opens only blockers, R-93-m1e); close this note with --moved once it is open (moved from a-test-helper-dies-with-its-test)"
- OpenedAt: 2026-11-03T16:10:17Z
- Revision: 4
- BlockedBy: a-test-helper-dies-with-its-test
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-03T16:10:27Z revision=2 opid=25FKMDCM3F7HMSW24F7YM1DVRK-m1e-7189b09c authority=proven digest=a0dbb7b68062aeee4c264155cba1fe5373a5ae0c27d9cae42a365957b7c79aa8 episode=2

History:
- 2026-11-03T16:10:17Z 7WCB7NNECGNMVDJHCV862ZCQPT-m1e-7189b09c open actor=human:Wido targets=every-detached-test-helper-is-in-custody,a-test-helper-dies-with-its-test
- 2026-11-03T16:10:27Z 25FKMDCM3F7HMSW24F7YM1DVRK-m1e-7189b09c approve actor=human:Wido targets=every-detached-test-helper-is-in-custody
- 2026-11-03T16:33:13Z YT4Z7G6TW7JMD603BJR6SS9N2D-ui-78aafdc1 read-items-close actor=human:Wido targets=a-test-helper-dies-with-its-test,every-detached-test-helper-is-in-custody
- 2026-11-03T22:57:44Z NMRB08AJCPD0PD6HG3AB9PCWRC-ui-71c5cb39 unpark actor=ui+steward-seat targets=every-detached-test-helper-is-in-custody reason=blocker a-test-helper-dies-with-its-test is done; the park lifts
Integrity: sha256=537882a4c4e4a661188407fb75aea35f3b8abc76934afef405f70fe4dfb2d1fa
