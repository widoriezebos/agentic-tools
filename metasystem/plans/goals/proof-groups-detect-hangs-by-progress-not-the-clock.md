# proof-groups-detect-hangs-by-progress-not-the-clock

- State: claimed
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: a correct candidate is refused, or a truly hung group runs longer before it is caught, nothing granted or destroyed; novelty 2: progress-based supervision of a child is new in the adapter, the event stream and process accounting exist; exposure 3: every landing receipt on every seat; accumulation 1: one adapter and one runner, no compounding"
- Tier: 3
- Intent: Wido, 2026-09-10 13:45Z: 'we have a test that is load dependent. That is not a test at all. Remove the entire timeout. We need to replace it with something else, something that is not load dependent.' Today the receipt's go adapter (internal/proofrun/test_go.go) runs every unit group under Go's ten-minute default -timeout, the shell sections run under wall-clock harness caps (scripts/agents/fixture-budget.sh), and one supervision fixture asserts a two-second census freshness; on 2026-09-10 the m1 seat, sharing its host with three other seats at load 6 to 12, saw goal-full-coverage fail at 600.3 s twice with every test passing and the census fixture refuse at its window, so correct candidates were refused by the clock. Done means: no proof group and no fixture bed is bounded by wall-clock time; a group is judged hung only by absence of progress, meaning no test event on the -json stream and no growth of the child process group's CPU time over a long window, and then killed with a goroutine dump and failed with that reason; a slow group under load passes; the group record names the progress rule it ran under; and a fixture that plays a starved host (a group that makes CPU progress but takes far longer than its target) passes while a fixture that plays a deadlocked test is failed as hung.
- Origin: main
- Next step: Revision 3 folded the second critique as decisions (implementation-first ruling); DONE narrowed to the receipt's groups and attempt (slices 1 and 2); fixture waits are goal fixture-waits-name-their-producer. Next: Sol builds slice 1 from the build brief (supervisor with dead/stopped/waiting/runaway verdicts by consumption and task state, optional protected cpuBudgetSeconds, -timeout 0), Opus critiques, land; then slice 2 (attempt deadline and the remaining engine contexts).
- OpenedAt: 2026-09-10T13:40:12Z
- Revision: 24
- Budget: elapsedLimit=3d attemptLimit=48 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 3
- Approved: by=human:Wido at=2026-09-11T09:54:10Z revision=22 opid=QY1MYNZNDWHY57N6SFZ1M84G90-m1-c6925449 authority=proven digest=dad2e53d5dda53ed598a102ccd9d0638dc5a4b0c2d672c25c9bf01ac9811b7bc
- Sliced: machine=m1 lineage=main-1788940932-18533-7fa6c2 revision=5 at=2026-09-10T14:14:54Z
- Claimed: machine=m1e lineage=main-1789030447-51011-5722fc at=2026-09-11T13:52:22Z revision=24 accountingRevision=24 episodeAt=2026-09-11T13:52:22Z episodeRevision=24
- StopCapability: generation=24 revision=24 machine=m1e claimEpoch=1 fenceEpoch=0

History:
- 2026-09-10T13:40:12Z 9E6AJGS1CME0CPWS26NAGGCS5J-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T13:40:35Z 5V7PAWS77HJNSVW2GKJFPCJMR6-m1-c6925449 approve actor=human:Wido targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T13:40:41Z BBFTT4VJ1Q7R300R1GQ4QTCF4X-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T13:46:52Z 8S4CN12MFTC38EVBH3F91E75ZA-m1-1701c13c release actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T14:14:31Z KCR8N6Y9K8SX767BX658GA3Z79-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T14:14:54Z ADG3S62X97Q5BH3RKVV2TZSDET-m1-1701c13c slice-start actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T14:37:30Z F22ZJNT3YBN5FWG6BZK9HJYYFK-m1-1701c13c release actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T20:48:39Z ZHXN3S0AKJDQRW9HQJRJ8VS904-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T20:59:16Z RTJXFTEVD7XK9BS19V0BP3KBBJ-m1-1701c13c release actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T21:00:39Z 4A25MT5PTTGR93VS8JSF8WES1K-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T21:04:49Z 08HQG32FXDRCSBT7CSRXN3BSR0-m1-1701c13c release actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T22:07:33Z ZPJKFW1GV7WJ7ZYE20T1C098T0-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T22:10:43Z F6M9MXBTPWXXPHA9N7C56GC9KP-m1-1701c13c release actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T22:29:09Z ZDTE933ER108KBN8SAHWYXNRK1-m1-1701c13c edit actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T22:29:27Z K5CKP8JPB7KMEE117TFEY163S2-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T22:31:12Z 6W6GNYPJMQ4RW5KPK934JZ2M8K-m1-1701c13c release actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T22:31:26Z R813S75D124S789ZWDSHWKKEMP-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T22:33:36Z 8DJB23G051VN7DJVWDFHFSHTKD-m1-1701c13c release actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-10T22:37:29Z VCGRWAKR7KHCMPNV5TVNR4T337-m1-1701c13c claim actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-11T00:34:18Z BQAQ501W1XWTRZSRSP7GZJKS5V-m1-c6925449 set-budget actor=human:Wido targets=proof-groups-detect-hangs-by-progress-not-the-clock displaced=m1+main-1788940932-18533-7fa6c2@2026-09-10T22:37:29Z
- 2026-09-11T01:15:34Z 87JWTKBBYTX3AC574KEW5CQMG6-m1-c6925449 set-budget actor=human:Wido targets=proof-groups-detect-hangs-by-progress-not-the-clock displaced=m1+main-1788940932-18533-7fa6c2@2026-09-11T00:34:18Z
- 2026-09-11T09:54:10Z QY1MYNZNDWHY57N6SFZ1M84G90-m1-c6925449 set-budget actor=human:Wido targets=proof-groups-detect-hangs-by-progress-not-the-clock displaced=m1+main-1788940932-18533-7fa6c2@2026-09-11T01:15:34Z
- 2026-09-11T13:12:46Z EP76YEAF9FNDJ474Q4MKSYR87A-m1-1701c13c release actor=m1+main-1788940932-18533-7fa6c2 targets=proof-groups-detect-hangs-by-progress-not-the-clock
- 2026-09-11T13:52:22Z S2R40KWSVJ8NQA8KJR2ER8E8QN-m1e-892cdaec claim actor=m1e+main-1789030447-51011-5722fc targets=proof-groups-detect-hangs-by-progress-not-the-clock
Integrity: sha256=b6caa6a5fe1eb693b530ce524e1f33c2d5bcfbc88612a6d8ccf5bc14960eac0b
