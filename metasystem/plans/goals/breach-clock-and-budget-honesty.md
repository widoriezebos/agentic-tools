# breach-clock-and-budget-honesty

- State: queued
- Intent: Wido's highest-priority order (2026-09-01, verbatim: 'This needs to be fixed immediately... Only resume work after these problems are fixed, proven with tests'): two of the three proven breach-machinery breakdowns from the night of 2026-08-31, sharing the goal-machinery seam. (1) THE RAISE-RESET CLOCK: SetBudget re-binds the claim record on every raise and the elapsed breach clock anchors on the current revision's claim timestamp - every budget raise restarts the breach clock (the night reset it five times lawfully; internal/goal/verbs.go SetBudget comment + internal/dispatch/budget.go anchor are the proof). (2) DISHONEST DURATIONS: budget elapsed limits parse through a working-hours grammar (d = 8 hours) and New() normalizes inputs into it - a human's 24h displays as 3d, and a human's 9d is enforced at 72 clock hours, one third of intent, silently, across every live budget.
- Origin: main
- Next step: Appetite: one 4h slice, Sol builds + Fable critiques. Fix 1: the elapsed clock anchors on the claim episode's ORIGINAL moment, surviving raises; only release-and-reclaim or an explicit human re-time restarts it; tests prove raises cannot outrun the breaker. Fix 2: for budget elapsed limits h=clock hour, d=24 clock hours, inputs stored verbatim; legacy canonical strings stay readable; the live budget records re-set from each human's recorded verbatim word (verify each against its goal history). SEAM: internal/goal, internal/goalbudget, internal/dispatch/budget.go - m2's claim; m3 works the disjoint steward-seam tripwire (burn-without-delivery-tripwire) in parallel. CLAIMED under Wido's fix-first order as the budget word. FIX 3 FOLDED IN (same goal seam, m3's wedge specimen from records/misc/idle-loss-2026-09-01.md): a breach-stop parks the GOAL, never the machine - release must succeed on a breach-stopped claim and the one-claim quota must not count a breach-stopped goal against its machine; hard deterministic Go in the goal verbs with tests proving a wedged machine frees itself. ALL THREE per Wido's word: hard deterministic machinery, Go enforcing behavior, no conduct.
- OpenedAt: 2026-09-01T06:54:30Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=360 activeJobLimit=1

History:
- 2026-09-01T06:54:30Z XQ8RYAX5R7JBZ9DH0TX694ENCA-m2-bc1be9cb open actor=m2+mac-coordinator targets=breach-clock-and-budget-honesty
- 2026-09-01T06:55:38Z 92DH70PXTT5QZTPCC72369W4M4-m2-bc1be9cb set-budget actor=human:wido targets=breach-clock-and-budget-honesty
- 2026-09-01T06:57:39Z RS2MPQXRHNER939N9HTSME2QKR-m2-bc1be9cb edit actor=m2+mac-coordinator targets=breach-clock-and-budget-honesty
Integrity: sha256=7ce81e854532ae0e518be717d1362b6d916e58c92459e9b3d5865aae09c64a76
