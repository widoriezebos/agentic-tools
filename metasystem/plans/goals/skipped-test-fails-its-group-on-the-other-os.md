# skipped-test-fails-its-group-on-the-other-os

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a correct candidate is refused at the gate and the seat routes around it, nothing granted or destroyed; novelty 1: one classification in the existing group verdict; exposure 2: every package with a platform-gated test, on every landing that selects it; accumulation 1: each refusal costs one receipt run"
- Tier: 2
- Intent: The proof run counts a skipped expected test as a failed group (internal/proofrun/test_build.go, the status check near line 755), so a package with an OS-gated skip can never pass its own delivery group on the other operating system. Seen 2026-09-10 09:58Z on m1: the new humanauthority-standard group ran go test -json ./internal/humanauthority with exit status 0, 45 observed tests, none failed, one skipped (TestLinuxDoesNotAdmitLoginWithWithheldArguments skips outside Linux), and the receipt reported the group failed with an empty reason and sufficient=false. The same package skips three Darwin-only tests on Linux, so on main it passes nowhere. Done means: a skip that the test itself declares as platform-gated does not fail the group, the group record names it as skipped with its reason, and a skip that is not platform-gated still fails the group; the receipt's TEST-GROUP line prints the reason instead of an empty reason= field.
- Origin: main
- Next step: m1c's lane (proof engine): read the skip reason from the go test json output, classify platform-gated skips (runtime.GOOS guards, build-tag absence) as expected omissions in the group record, keep other skips as failures, and print the reason on the TEST-GROUP line; Sol builds behind a fixture that plays one gated and one ungated skip, Opus critiques.
- OpenedAt: 2026-09-10T09:59:25Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-10T09:59:25Z YS9E1DBHFWFR7NZ4EY5JEJHZ27-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=skipped-test-fails-its-group-on-the-other-os
Integrity: sha256=d3e60773913092a1e2f84dbbed2bb53bdafdc89921db28316b53d5dcd22f8a6c
