# tests-observe-only-what-they-inject-2

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="The same design and the same answers as tests-observe-only-what-they-inject: test-only changes to how fixtures observe processes; a wrong tag or audit rule hides a leftover (severity 3), the seam exists (novelty 2), every package's tests run under it (exposure 3), each unit adds a detector that stays (accumulation 3)."
- Tier: 3
- Intent: Tests observe only what they inject, part 2: every detached arming helper carries the binary-owned tag and a death witness proves that a dying supervise test binary leaves no helper behind (U2); an audit with a shrink-only baseline and reasons holds the line so no new violation lands (U3).
- Origin: main
- Next step: Brief U2 from the design page's inventory (internal/supervise/arming_test.go, 0 production lines expected); Codex builds it in a registered workspace, Opus reads, sharded gate, hand landing; then U3.
- OpenedAt: 2026-10-19T15:09:36Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-19T15:09:36Z ME87ZJPNYV87M9VVW1PSZFR47H-m1e-f2f350ed open actor=human:Wido targets=tests-observe-only-what-they-inject-2
Integrity: sha256=cf0b9ab64150fabfd08e731a9d4970d8bc8ae6ff4116da9482eb47edd54d95e7
