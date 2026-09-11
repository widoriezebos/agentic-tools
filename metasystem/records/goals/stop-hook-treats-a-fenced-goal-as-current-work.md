# stop-hook-treats-a-fenced-goal-as-current-work

- State: done
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: the seat cycles refusals it cannot answer and the human is sent at commands the record proves refused; novelty 1: one state check in the current-goal branch of the turn verdict; exposure 3: every seat whose held goal ever breach-stops; accumulation 2: ten refusals in one session on m1d, each re-quoting a stale ask"
- Tier: 3
- Intent: The Stop hook's NOTHING LEFT TO WORK ON branch reads the machine's held goal as the seat's current work even when that goal is breach-stopped and unresumable, and quotes its next step as the thing to do now. On m1d, 2026-09-09, ten consecutive stop refusals quoted account-provenance's 2026-09-07 next step, an ask for a budget raise that records/misc/account-provenance-resume-is-wedged.md proves is refused by the fence, while every claim-clearing verb on it refused. stop-hook-open-work-refusal-repeats (done) fixed the once-only promise for plan lines; this is the goal-next-step branch, where the promise never held, and the seat could not record why it was blocked where the hook reads because editing a dead lineage's claim is a human act. DONE means a held goal with a standing StopFence is reported as fenced, with the stop id and reason, never as current work with its next step; the refusal for that state does not repeat across turns; and a fixture proves a fenced held goal yields one fenced line and a second stop on the same state passes.
- Origin: main
- Next step: ONE MORE LINE FROM THE CLOSING READ OF breach-stop-wedges-seat (BSW-19, 2026-09-10): in the turn verdict's normal branch the early exit for 'WORK IN FLIGHT ... claimable shared backlog also includes' returns before the FENCED lines are appended, so in that state a stopped claim beside the live one is not shown; the verdict's other branches show it. Fold when this goal builds. Appetite: 1h. If breach-stop-wedges-seat lands the shared predicate in convertedGoalFacts (internal/goal/turnverdict.go), what remains is the once-only marker for the goal branch: extend the seen-state marker that stop-hook-open-work-refusal-repeats added for plan lines to the fenced-goal line. Canary: fixture a held goal into the account-provenance shape (claimed, fenced, stale batch), run the hook twice, assert one FENCED line then a pass.
- Concluded: Absorbed by goal:stop-hook-never-forces-an-empty-turn in the 2026-09-11 backlog consolidation on Wido's word; The reporting half landed with breach-stop-wedges-seat (2f764c60): internal/goal/turnverdict.go:757 FencedClaimLines prints 'FENCED <goal>: breach-stopped by <stop> (<reason>); only goal resume ... clears it; the queue is open'; the once-only marker exists only for plan lines (47e5f2f2) and session . Its specific requirement is appended to that goal's next step.
- OpenedAt: 2026-09-09T09:31:12Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-09T09:31:12Z 2B9E7KNCDQF2Y1DVA39GYRCEH3-m1-c6925449 open actor=human:Wido targets=stop-hook-treats-a-fenced-goal-as-current-work
- 2026-09-10T10:40:09Z 2XG6YCJ51SN77VD5J8RWA9WRZA-m1d-a38bcdde edit actor=m1d+main-1788941004-20871-6e7a43 targets=stop-hook-treats-a-fenced-goal-as-current-work
- 2026-09-11T22:06:08Z MPV0Z1ARH997HP5JH22K8RAE2Y-m1-c6925449 done actor=human:Wido targets=stop-hook-treats-a-fenced-goal-as-current-work
Integrity: sha256=859ad8ad2595d51e716c800040b9b0d8055a90345144097ca9b061ae6c383966
