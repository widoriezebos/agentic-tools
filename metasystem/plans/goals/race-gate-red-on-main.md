# race-gate-red-on-main

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=3 basis="The full Go gate the adopt fixture runs (go test -race -cover -timeout 30m ./internal/...) is red on plain main on a quiet Mac in four packages; every landing that relies on that gate is exposed; the debt grows with each landing that does not run it."
- Tier: 3
- Intent: scripts/adopt-fixtures.sh runs go test -race -cover -timeout 30m ./internal/... (scripts/agents/go-gate.sh line 528). On m2 2026-09-04 21:13Z, on a quiet Mac at f40fcf50, that gate was red in four packages: internal/goal and internal/missionrunner timed out at 30 minutes under the race detector (18 and 24 minutes without it), internal/refusal failed TestHCL03EveryCodeRowed, and internal/steward failed TestArmConfirmsTheGuardAndDisarmEndsIt after 1042 seconds. The fast gate (go-gate.sh --fast) that landings run does not include this, so the debt is invisible at landing. DONE means the two timeouts are addressed (a per-package budget that fits the goal and mission-runner packages under -race, or those packages' slow tests marked and run in a separate long lane the adopt fixture invokes), the refusal and steward failures are fixed at their cause, and the adopt fixture is green on a quiet Mac; the evidence log of this run is at the seat's scratchpad and the failing test names are in this intent.
- Origin: main
- Next step: Read the four failures, split into a budget change and two test fixes; tier from the risk basis; waits for Wido's word if above tier 1.
- OpenedAt: 2026-09-04T21:14:28Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-04T21:14:28Z WPG34JGC5TNCHRKWTV7W41BGT4-m2-5fcf08ab open actor=human:Wido targets=race-gate-red-on-main
Integrity: sha256=40879ed33ae5105c7aa69631e9901ccd44bcc38bda269ee53574d18cbe382036
