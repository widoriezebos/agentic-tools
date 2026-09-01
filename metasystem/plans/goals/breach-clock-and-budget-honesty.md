# breach-clock-and-budget-honesty

- State: queued
- Intent: Wido's highest-priority order (2026-09-01, verbatim: 'This needs to be fixed immediately... Only resume work after these problems are fixed, proven with tests'): two of the three proven breach-machinery breakdowns from the night of 2026-08-31, sharing the goal-machinery seam. (1) THE RAISE-RESET CLOCK: SetBudget re-binds the claim record on every raise and the elapsed breach clock anchors on the current revision's claim timestamp - every budget raise restarts the breach clock (the night reset it five times lawfully; internal/goal/verbs.go SetBudget comment + internal/dispatch/budget.go anchor are the proof). (2) DISHONEST DURATIONS: budget elapsed limits parse through a working-hours grammar (d = 8 hours) and New() normalizes inputs into it - a human's 24h displays as 3d, and a human's 9d is enforced at 72 clock hours, one third of intent, silently, across every live budget.
- Origin: main
- Next step: Appetite: one 4h slice, Sol builds + Fable critiques. Fix 1: the elapsed clock anchors on the claim episode's ORIGINAL moment, surviving raises; only release-and-reclaim or an explicit human re-time restarts it; tests prove raises cannot outrun the breaker. Fix 2: for budget elapsed limits h=clock hour, d=24 clock hours, inputs stored verbatim; legacy canonical strings stay readable; the live budget records re-set from each human's recorded verbatim word (verify each against its goal history). SEAM: internal/goal, internal/goalbudget, internal/dispatch/budget.go - m2's claim; m3 works the disjoint steward-seam tripwire (burn-without-delivery-tripwire) in parallel. CLAIMED under Wido's fix-first order as the budget word.
- OpenedAt: 2026-09-01T06:54:30Z
- Revision: 1

History:
- 2026-09-01T06:54:30Z XQ8RYAX5R7JBZ9DH0TX694ENCA-m2-bc1be9cb open actor=m2+mac-coordinator targets=breach-clock-and-budget-honesty
Integrity: sha256=31fc2ffdb6210c5f50b97ba809f9bb4b5a709a9ecf2d48abe166f0051c7ec269
