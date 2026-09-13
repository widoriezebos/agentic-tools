# skipped-test-fails-its-group-on-the-other-os

- State: approved
- Priority: 1
- Sequence: 30
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a correct candidate is refused at the gate and the seat routes around it, nothing granted or destroyed; novelty 1: one classification in the existing group verdict; exposure 2: every package with a platform-gated test, on every landing that selects it; accumulation 1: each refusal costs one receipt run"
- Tier: 2
- Intent: The proof run counts a skipped expected test as a failed group (internal/proofrun/test_build.go, the status check near line 755), so a package with an OS-gated skip can never pass its own delivery group on the other operating system. Seen 2026-09-10 09:58Z on m1: the new humanauthority-standard group ran go test -json ./internal/humanauthority with exit status 0, 45 observed tests, none failed, one skipped (TestLinuxDoesNotAdmitLoginWithWithheldArguments skips outside Linux), and the receipt reported the group failed with an empty reason and sufficient=false. The same package skips three Darwin-only tests on Linux, so on main it passes nowhere. Done means: a skip that the test itself declares as platform-gated does not fail the group, the group record names it as skipped with its reason, and a skip that is not platform-gated still fails the group; the receipt's TEST-GROUP line prints the reason instead of an empty reason= field.
- Origin: main
- Next step: m1c's lane (proof engine): read the skip reason from the go test json output, classify platform-gated skips (runtime.GOOS guards, build-tag absence) as expected omissions in the group record, keep other skips as failures, and print the reason on the TEST-GROUP line; Sol builds behind a fixture that plays one gated and one ungated skip, Opus critiques. Named cases from the join critic ha-surfacecrit2-20260910 (HAS-05, HAS-06, 2026-09-10): after the hp-terminal landing, the subtest 'Terminal app login is the session leader' of TestTerminalAppLoginAndTmuxSessionShapesEnroll in internal/humanauthority/authority_test.go still skips outside Darwin with a stale reason (the per-OS login program list it names was deleted; the check now stats the real /usr/bin/login as a root-owned write-protected file, which exists on Debian too), so the first Linux run under this goal retries it without its OS guard; and TestRootAncestorWithWithheldArgumentsHasReadableIdentityFacts in internal/identity/enumerate_test.go passes only under a root-owned withheld-argument ancestor such as Terminal.app's login, so runtime-owner-standard fails on any Linux seat and on a daemon or tmux-only ancestry on this Mac.
- OpenedAt: 2026-09-10T09:59:25Z
- Revision: 5
- Pinned: m1c
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-13T06:40:19Z revision=4 opid=SR5BH6HZM2ZHRHTYF6T0DRE8GA-m1e-c6925449 authority=proven digest=a0a43ad70d8f7fe7efe8e1f234f44f8e0170aba804b0616fffc7ae29b76560b1

History:
- 2026-09-10T09:59:25Z YS9E1DBHFWFR7NZ4EY5JEJHZ27-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=skipped-test-fails-its-group-on-the-other-os
- 2026-09-10T10:31:40Z MXT39VVET2TP1XMMPQFVD95ZN1-m1-1701c13c edit actor=m1+main-1788940932-18533-7fa6c2 targets=skipped-test-fails-its-group-on-the-other-os
- 2026-09-13T06:40:13Z 42WAD1J3SW9FQ01MDSWFEJ8M4F-m1e-c6925449 set-pin actor=human:Wido targets=skipped-test-fails-its-group-on-the-other-os
- 2026-09-13T06:40:19Z SR5BH6HZM2ZHRHTYF6T0DRE8GA-m1e-c6925449 approve actor=human:Wido targets=skipped-test-fails-its-group-on-the-other-os
- 2026-09-13T06:44:13Z Q70ZB0788G30WT1XM8BHQ367D2-m1e-c6925449 set-priority actor=human:Wido targets=skipped-test-fails-its-group-on-the-other-os reason=priority-order subject=skipped-test-fails-its-group-on-the-other-os from=unranked to=1:30 requested-sequence=30
Integrity: sha256=bbc067ec6f47cbbdb25a705e5df1aff0869a3b09743b8dcb68958a3dae3737de
