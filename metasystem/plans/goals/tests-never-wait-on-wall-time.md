# tests-never-wait-on-wall-time

- State: queued
- Priority: 2
- Sequence: 29
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: a wall-clock guard that flakes under load turns a green tree red and a red tree green at random, and a flake reported as green hides a defect; novelty 1: a source-level check and a rerun rule in the existing gate; exposure 3: every Go test in every package; accumulation 1: nothing builds on the rule itself"
- Tier: 2
- Intent: Delivery-efficiency phase D, delegate loop. The testing contract refuses a Go test that waits on wall-clock time for a condition to become true (a sleep, a timer, or a deadline used as a hang guard around a fake-clock condition), and the gate treats a test that fails and then passes on rerun as a patience finding with its load reading, never as a green. Why: on 2026-09-14 the stop-decisions build's hook bed went red on a one-second wall-clock guard added in round 10, under a load average above five, while the code under test was unchanged and the subtest passed five reruns in a row; under R-35-m3 a load-caused failure is a patience defect and must be attributable, and a wall-clock guard is a flake source that will recur on every loaded machine. DONE: a source contract names the constructs a test may not use to wait and the gate refuses a test that uses them; a fixture test with a wall-clock guard is refused with the line named; the gate records fail-then-pass reruns as findings with the load reading, proven by a fixture. every mechanism in this goal lives in the Go engine, the ledger verbs and the adapter contract, never in one runtime; it is proven on at least two runtimes.
- Origin: human
- Next step: Design page: the forbidden constructs, the source check, the rerun record and its fixture; then build.
- OpenedAt: 2026-09-14T16:17:21Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-14T16:17:21Z BB2BF7NWX7DTTDZNB10KRZFY7P-m1e-c6925449 open actor=human:Wido targets=tests-never-wait-on-wall-time
- 2026-09-14T16:17:26Z 5ZQ3VSK1JRJ8V7E25FFYYW5T0K-m1e-c6925449 set-priority actor=human:Wido targets=tests-never-wait-on-wall-time reason=priority-order subject=tests-never-wait-on-wall-time from=unranked to=2:29 requested-sequence=append
Integrity: sha256=8478daab2d39be0d181b12b20fccafd3ea2ba12f1f193c27572f1ea346d1ebe4
