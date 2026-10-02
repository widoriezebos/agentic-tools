# lane-check-red

- State: abandoned
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="test-only change meant to be returned"
- Tier: 1
- Intent: Lane real-run check (2026-10-02 night): a deliberately failing test handed to the plain lane; the landing agent must find it as the culprit, return it to the seat and land the rest; main must stay green. Never lands.
- Origin: human
- Next step: hand in with work land; expect it returned
- OpenedAt: 2026-10-02T02:10:22Z
- Revision: 4
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-02T02:10:33Z revision=2 opid=HVMQQ5RP1F8WNY5VDSXG67P8BY-m1e-9c612d71 authority=proven digest=8fd2c9635052c92d5f33dbcca71de5b132804bfcde3649a700f6962a1ccf3410 episode=2
- Abandoned: by=human:Wido at=2026-10-02T02:49:36Z revision=4 opid=MDYA9CYSVJ1826WH6MZ1EXYN1Z-m1e-9c612d71 displaced=m1e+main-1790454088-93948-21671b@2026-10-02T02:11:20Z because=deliberately red check branch; returned by the lane as designed (batch check 2026-10-02)

History:
- 2026-10-02T02:10:22Z M8Y22NTERJ0KAJNH3CREPMT09J-m1e-9c612d71 open actor=human:Wido targets=lane-check-red
- 2026-10-02T02:10:33Z HVMQQ5RP1F8WNY5VDSXG67P8BY-m1e-9c612d71 approve actor=human:Wido targets=lane-check-red
- 2026-10-02T02:11:20Z ZSKR1ETV7TYKGTVBTSH4P460B1-m1e-f456f182 claim actor=m1e+main-1790454088-93948-21671b targets=lane-check-red
- 2026-10-02T02:49:36Z MDYA9CYSVJ1826WH6MZ1EXYN1Z-m1e-9c612d71 abandon actor=human:Wido targets=lane-check-red displaced=m1e+main-1790454088-93948-21671b@2026-10-02T02:11:20Z reason=deliberately red check branch; returned by the lane as designed (batch check 2026-10-02)
Integrity: sha256=1d84850780b40503c7438d58728f35b5453463b1d36acdac93a289531b0ca1ab
