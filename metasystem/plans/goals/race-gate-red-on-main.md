# race-gate-red-on-main

- State: approved
- Risk: severity=2 novelty=1 exposure=3 accumulation=3 basis="The full Go gate the adopt fixture runs (go test -race -cover -timeout 30m ./internal/...) is red on plain main on a quiet Mac in four packages; every landing that relies on that gate is exposed; the debt grows with each landing that does not run it."
- Tier: 3
- Intent: scripts/adopt-fixtures.sh runs go test -race -cover -timeout 30m ./internal/... (scripts/agents/go-gate.sh line 528). On m2 2026-09-04 21:13Z, on a quiet Mac at f40fcf50, that gate was red in four packages: internal/goal and internal/missionrunner timed out at 30 minutes under the race detector (18 and 24 minutes without it), internal/refusal failed TestHCL03EveryCodeRowed, and internal/steward failed TestArmConfirmsTheGuardAndDisarmEndsIt after 1042 seconds. The fast gate (go-gate.sh --fast) that landings run does not include this, so the debt is invisible at landing. DONE means the two timeouts are addressed (a per-package budget that fits the goal and mission-runner packages under -race, or those packages' slow tests marked and run in a separate long lane the adopt fixture invokes), the refusal and steward failures are fixed at their cause, and the adopt fixture is green on a quiet Mac; the evidence log of this run is at the seat's scratchpad and the failing test names are in this intent.
- Origin: main
- Next step: Read the four failures, split into a budget change and two test fixes; tier from the risk basis; waits for Wido's word if above tier 1.
- OpenedAt: 2026-09-04T21:14:28Z
- Revision: 4
- Labels: robustness
- Pinned: m1c
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=0e61f641bca538c7f64dc22b09fa17c6da5e8650f292db27090b5c9be242ae8f

History:
- 2026-09-04T21:14:28Z WPG34JGC5TNCHRKWTV7W41BGT4-m2-5fcf08ab open actor=human:Wido targets=race-gate-red-on-main
- 2026-09-04T22:11:09Z ZH0H4W45RP14R7MMS4P85XFNNY-m2-5fcf08ab approve actor=human:Wido targets=race-gate-red-on-main authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="Yes, all five (Recommended)"
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=race-gate-red-on-main reason=sweep
- 2026-09-06T07:28:59Z 3ZZ8AFSPB0S2BDG1T28CFSVY38-m1-a4f8999f set-pin actor=human:Wido targets=race-gate-red-on-main
Integrity: sha256=5be7576176d5a53ea7738e5c1cd9b9e09ed849e90aeb73720ee26a3b277d216e
