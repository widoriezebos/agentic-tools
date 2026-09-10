# proof-groups-detect-hangs-by-progress-not-the-clock

- State: approved
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: a correct candidate is refused, or a truly hung group runs longer before it is caught, nothing granted or destroyed; novelty 2: progress-based supervision of a child is new in the adapter, the event stream and process accounting exist; exposure 3: every landing receipt on every seat; accumulation 1: one adapter and one runner, no compounding"
- Tier: 3
- Intent: Wido, 2026-09-10 13:45Z: 'we have a test that is load dependent. That is not a test at all. Remove the entire timeout. We need to replace it with something else, something that is not load dependent.' Today the receipt's go adapter (internal/proofrun/test_go.go) runs every unit group under Go's ten-minute default -timeout, the shell sections run under wall-clock harness caps (scripts/agents/fixture-budget.sh), and one supervision fixture asserts a two-second census freshness; on 2026-09-10 the m1 seat, sharing its host with three other seats at load 6 to 12, saw goal-full-coverage fail at 600.3 s twice with every test passing and the census fixture refuse at its window, so correct candidates were refused by the clock. Done means: no proof group and no fixture bed is bounded by wall-clock time; a group is judged hung only by absence of progress, meaning no test event on the -json stream and no growth of the child process group's CPU time over a long window, and then killed with a goroutine dump and failed with that reason; a slow group under load passes; the group record names the progress rule it ran under; and a fixture that plays a starved host (a group that makes CPU progress but takes far longer than its target) passes while a fixture that plays a deadlocked test is failed as hung.
- Origin: main
- Next step: Slice 1, this seat: design revision 1 by the seat (plans/proof-groups-progress-hang-design.md), Sol critiques; then Sol builds the go adapter: -timeout 0, streaming -json supervision, progress = events or CPU time of the process group (ps -o cputime -g), hung = no progress for the window (a constant, not a config key), SIGQUIT then SIGKILL, reason on the record; Opus critiques; land. Slice 2: the section runner and fixture-budget.sh caps become progress bounds the same way. The census freshness fixture is goal supervision-fixture-census-window-fails-under-load. The parked goal go-groups-carry-their-target-as-the-test-timeout is superseded.
- OpenedAt: 2026-09-10T13:40:12Z
- Revision: 13
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-10T13:40:35Z revision=2 opid=5V7PAWS77HJNSVW2GKJFPCJMR6-m1-c6925449 authority=proven digest=999d6933823998abfed59fcbbffdffbd547ce9a97bad19e899482298636ffbf1
- Sliced: machine=m1 lineage=main-1788940932-18533-7fa6c2 revision=5 at=2026-09-10T14:14:54Z

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
Integrity: sha256=b0eb2398d15243a73c37e2c55f2acf27e4a40355c81b0c61f7789315ba5535dc
