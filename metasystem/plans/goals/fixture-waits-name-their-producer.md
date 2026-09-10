# fixture-waits-name-their-producer

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="severity 2: a correct candidate refused at the gate by host load, nothing granted or destroyed; novelty 2: an owned-producer contract for shell waits is new; exposure 2: every landing whose plan selects a fixture bed; accumulation 1: one helper, migrated bed by bed"
- Tier: 2
- Intent: The shell fixture beds under scripts/agents bound their waits with SECONDS deadlines and harness caps (fixture-budget.sh: every cap at least ten times the expected duration); under host load those deadlines refuse correct candidates, which Wido ruled out on 2026-09-10 (a load-dependent test is not a test). The second critique of the hang-detection design (PHD-FIXTURE-WAIT-SEMANTICS) showed that a generic consumption supervisor cannot replace these waits: a loop that waits for a file, a lock handoff, an attestation, a mission state or a producer's exit sleeps and consumes nothing while another process owns the transition, so only that producer's liveness can bound the wait. Done means: every fixture wait names the producer it waits on and ends only when that producer exits, is judged hung by the proof run's progress rules, or delivers the state; no SECONDS deadline or harness cap decides a fixture outcome; fixture-budget.sh becomes the owner of producer contracts rather than of seconds.
- Origin: main
- Next step: After proof-groups-detect-hangs-by-progress-not-the-clock lands its supervisor: design one wait contract (producer pid, expected transition, failure signal) and the shell helper that implements it; migrate the beds one at a time, dispatch-fixtures.sh (forty-six deadlines) last.
- OpenedAt: 2026-09-10T22:29:05Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-10T22:29:05Z PM4QNHYAGETYRZ5V8J36YF6APB-m1-1701c13c open actor=m1+main-1788940932-18533-7fa6c2 targets=fixture-waits-name-their-producer
Integrity: sha256=0cfc0e5ed595eac9f86f41a2d8fbbb303c68e4cd7ea1f7056a099dd908f53019
