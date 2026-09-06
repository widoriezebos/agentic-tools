# deadline-expired-register-row

- State: claimed
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: a missing register row fails one test and reddens gates, nothing unsafe is permitted; novelty 1: one exclusion line in a pattern that exists; exposure 1: one file, one token; accumulation 1: nothing compounds"
- Tier: 1
- Intent: Landing 19b14a9b (slice 2 of stop-hook-budget-is-ours, m1d) introduced the component outcome DEADLINE_EXPIRED without a row or exclusion in the refusal register (internal/refusal/register.go), so TestHCL03EveryCodeRowed fails on main ('collected refusal token DEADLINE_EXPIRED has no row or exclusion') and every full gate is red until it is fixed. Found by m1c, relayed by m1, verified on m1d at 1271fa38. The token is a steward component-evidence outcome that refuses nothing, exactly like AUTO_HEAL_ELIGIBLE, which the register excludes with that reason. DONE means the exclusion line exists beside AUTO_HEAL_ELIGIBLE with the same reason and the refusal package's tests pass on main.
- Origin: main
- Next step: TIER 1, one line: add the exclusion {Pattern: DEADLINE_EXPIRED, Reason: a steward health observation that refuses nothing} beside AUTO_HEAL_ELIGIBLE in internal/refusal/register.go; receipt: go test ./internal/refusal (GOTOOLCHAIN=go1.26.5 on this host). Build through a builder round and land through the tier-1 lane. The fast gate does not run this test; goal fast-gate-runs-the-refusal-register (m1c) adds it so this cannot recur.
- OpenedAt: 2026-09-06T15:29:44Z
- Revision: 4
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T15:31:09Z revision=2 opid=6VBFYTRR7Y9W7FZK222SCRWG4E-m1-7cd0bd60 authority=proven digest=4980025096946390f7b17ca7125079e9c251ca133b7e95adc727dda87782aa7a
- Sliced: machine=m1d lineage=main-1788683763-71870-f7f607 revision=3 at=2026-09-06T15:32:10Z
- Claimed: machine=m1d lineage=main-1788683763-71870-f7f607 at=2026-09-06T15:32:05Z revision=3 accountingRevision=3
- StopCapability: generation=3 revision=3 machine=m1d claimEpoch=1 fenceEpoch=0

History:
- 2026-09-06T15:29:44Z XK9BK99Y5KB2VTZXWKBW23S9B1-m1d-62183579 open actor=m1d+main-1788683763-71870-f7f607 targets=deadline-expired-register-row
- 2026-09-06T15:31:09Z 6VBFYTRR7Y9W7FZK222SCRWG4E-m1-7cd0bd60 approve actor=human:Wido targets=deadline-expired-register-row
- 2026-09-06T15:32:05Z M4YNKD70RZDTAB1JBS5A0DFM71-m1d-62183579 claim actor=m1d+main-1788683763-71870-f7f607 targets=deadline-expired-register-row
- 2026-09-06T15:32:10Z BWZATFZBJN286FHZ3SD04TTZ2F-m1d-62183579 slice-start actor=m1d+main-1788683763-71870-f7f607 targets=deadline-expired-register-row
Integrity: sha256=6005f3ae8a78ddd3ac5020531d2b2da25a2d9b375e74c7a36f1f61fa3739405b
