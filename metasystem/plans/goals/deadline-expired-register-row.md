# deadline-expired-register-row

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: a missing register row fails one test and reddens gates, nothing unsafe is permitted; novelty 1: one exclusion line in a pattern that exists; exposure 1: one file, one token; accumulation 1: nothing compounds"
- Tier: 1
- Intent: Landing 19b14a9b (slice 2 of stop-hook-budget-is-ours, m1d) introduced the component outcome DEADLINE_EXPIRED without a row or exclusion in the refusal register (internal/refusal/register.go), so TestHCL03EveryCodeRowed fails on main ('collected refusal token DEADLINE_EXPIRED has no row or exclusion') and every full gate is red until it is fixed. Found by m1c, relayed by m1, verified on m1d at 1271fa38. The token is a steward component-evidence outcome that refuses nothing, exactly like AUTO_HEAL_ELIGIBLE, which the register excludes with that reason. DONE means the exclusion line exists beside AUTO_HEAL_ELIGIBLE with the same reason and the refusal package's tests pass on main.
- Origin: main
- Next step: TIER 1, one line: add the exclusion {Pattern: DEADLINE_EXPIRED, Reason: a steward health observation that refuses nothing} beside AUTO_HEAL_ELIGIBLE in internal/refusal/register.go; receipt: go test ./internal/refusal (GOTOOLCHAIN=go1.26.5 on this host). Build through a builder round and land through the tier-1 lane. The fast gate does not run this test; goal fast-gate-runs-the-refusal-register (m1c) adds it so this cannot recur.
- OpenedAt: 2026-09-06T15:29:44Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-06T15:29:44Z XK9BK99Y5KB2VTZXWKBW23S9B1-m1d-62183579 open actor=m1d+main-1788683763-71870-f7f607 targets=deadline-expired-register-row
Integrity: sha256=192d8702f83644391dcc5af15e46cbed53d4dfefb5d3a147423e0910caa0df89
