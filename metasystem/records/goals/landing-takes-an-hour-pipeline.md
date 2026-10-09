# landing-takes-an-hour-pipeline

- State: done
- Priority: 1
- Sequence: 49
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="Every landing goes through it (exposure 3); a stale preparation is redone, never landed (severity 2); new scheduling in the lane (novelty 2); two units (accumulation 1)"
- Tier: 2
- Intent: While the lane proves one goal, the next waiting goal is merged with the expected main and passes its cheap tier, so it needs only its full proof when main moves (follow-up of landing-takes-an-hour)
- Origin: main
- Next step: Design from plans/landing-takes-an-hour-pipeline-design-brief.md after landing-takes-an-hour U1-U3; before switch-on
- Concluded: Landing takes an hour, minimal pipeline (P3): one goal per batch (landing.batch=1), every merge conflict returns the goal to its seat with its paths, the lane commits nothing. Accepted at the critique stop with known limits for the first person-supervised lane run; pipelining, multi-goal batches and lane-side fixes deferred with triggers. Landed by hand with goal 8.
- OpenedAt: 2026-10-08T17:40:28Z
- Revision: 6
- BlockedBy: landing-takes-an-hour
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-08T17:42:08Z revision=3 opid=TZQZNS0M8YF55R7R6CNKYMSBNA-m1e-718ba0eb authority=proven digest=0a91a1188565650e85d6189600bb8e6c55a5ff1ef5777b0be7c67729399c2049 episode=3

History:
- 2026-10-08T17:40:28Z 3DD5FNHHGYCXVDJFA3V5WA644T-m1e-718ba0eb open actor=human:Wido targets=landing-takes-an-hour-pipeline
- 2026-10-08T17:40:38Z 419BXASZ21N2BNJBPTJNHD9TQY-m1e-718ba0eb block actor=human:Wido targets=landing-takes-an-hour-pipeline reason=blocked by landing-takes-an-hour; returns when it is done
- 2026-10-08T17:42:08Z TZQZNS0M8YF55R7R6CNKYMSBNA-m1e-718ba0eb approve actor=human:Wido targets=landing-takes-an-hour-pipeline
- 2026-10-08T17:42:17Z 9SFXS775W64XJ8CZFXPZ4YWY74-m1e-718ba0eb set-priority actor=human:Wido targets=landing-takes-an-hour-pipeline reason=priority-order subject=landing-takes-an-hour-pipeline from=unranked to=1:50 requested-sequence=append
- 2026-10-09T14:54:47Z EQQ0PSBQ7RRH1NFZ8003AC1Q7F-m1e-718ba0eb unpark actor=human:Wido targets=landing-takes-an-hour,landing-takes-an-hour-pipeline reason=blocker landing-takes-an-hour is done; the park lifts; priority-order from=1:50 to=1:49
- 2026-10-09T14:55:06Z TVBZ7ZQDA48K6Z9XH423JHJ5QC-m1e-718ba0eb done actor=human:Wido targets=landing-takes-an-hour-pipeline
Integrity: sha256=3e2152461f0c04f87c7d928e7f2ec0bd1b6eeff2c043a40d792d5a8ea529730a
