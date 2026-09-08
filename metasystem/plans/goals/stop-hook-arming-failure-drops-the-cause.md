# stop-hook-arming-failure-drops-the-cause

- State: queued
- Priority: 3
- Sequence: 21
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: the seat cannot arm and cannot see why, so it either guesses or stalls, and a wedged seat is how sessions get killed by hand; novelty 1: the values are already returned by the call the hook makes, it is pass-through wording; exposure 3: every seat's every turn end runs this hook; accumulation 2: each occurrence costs the same rediscovery, and the cause is invisible in the transcript afterwards"
- Tier: 3
- Intent: When the Stop hook cannot arm supervision it says only 'Metasystem could not prove that stopping is safe: supervision arming failed', naming neither the cause nor the remedy, although the arming call it just made returns both. On m1d 2026-09-07 that line appeared for twenty minutes with no way to act on it; running 'metasystem up --repo .' by hand immediately printed the real answer - 'accepted-engine outcome=ENROLLMENT_DRIFT: rebuilt engine carries build stamp dev-e1e5fa37-dirty; automatic re-arm is bounded to landed commits' with the remedy naming steward restart at an agent-free terminal. The seat had rebuilt bin/metasystem from a dirty tree to verify a fix, which is ordinary work; the bound is deliberate (goal engine-rebuild-rearms-itself), so the refusal is correct and only its wording is wrong. A sibling goal already made this hook carry the goal-thread and HEALTH verdicts (stop-hook-refusal-carries-verdict, done); the arming branch was not covered. DONE means the arming-failure line carries the component outcome, the detail and the remedy exactly as 'up' prints them, so the reader can act without rerunning anything by hand.
- Origin: main
- Next step: Read the arming branch of scripts/agents/supervision-hook.sh against what metasystem up returns, and pass the outcome, detail and remedy through instead of collapsing them to 'supervision arming failed'. The fixture shape is a bed whose engine carries a dirty build stamp: the hook's refusal must name ENROLLMENT_DRIFT and the restart remedy, not a generic line. Worth checking the other collapsed branches in the same script while there.
- OpenedAt: 2026-09-07T09:18:11Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-07T09:18:11Z A0H2BDCN7YDAHRF72FSVVJ2MJP-m1d-25755dc0 open actor=m1d+main-1788764558-63534-a15b0d targets=stop-hook-arming-failure-drops-the-cause
- 2026-09-08T16:00:21Z YP6DSJNFBGNZCCC99MVB3CQWF9-m1-7cd0bd60 set-priority actor=human:Wido targets=stop-hook-arming-failure-drops-the-cause reason=priority-order subject=stop-hook-arming-failure-drops-the-cause from=unranked to=3:21 requested-sequence=21
Integrity: sha256=238df1ee4cffa88448b8645316a5e99469e3b8bb27ea1d8947f23aef4bd47b68
