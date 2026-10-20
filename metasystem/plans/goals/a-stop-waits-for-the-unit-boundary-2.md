# a-stop-waits-for-the-unit-boundary-2

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="The same design and the same answers as a-stop-waits-for-the-unit-boundary: a unit cut mid-work loses its work and its read (severity 3); the eligibility predicate and the grace join existing stop routes and admission checks (novelty 2); every elapsed breach on every seat passes them (exposure 3); one overrun record per breached unit on its goal (accumulation 2)."
- Tier: 3
- Intent: A stop waits for the unit boundary, part 2: after U1a.1 injected every probe on the runner's path (D12), U1a.2 makes every automatic elapsed-breach producer wait for the unit's durable handoff instead of cutting the unit (D9, BO-2: BreachStopEligible in FindBreachStops, the DEFERRED route, heldWorkCompleted), and U1b lets the breached unit finish its read and allowed corrections past the limit with the overrun recorded on the goal (D10: grace on the runner's authorization through the three admission checks, elapsedOverrun from the goal clock). Design: plans/designs/a-stop-waits-for-the-unit-boundary.md (accepted 2026-10-19; U1a.1 landed 2026-10-20 under the first goal).
- Origin: main
- Next step: Brief U1a.2 from the FINAL U1a brief's second unit (lane-test-20261009/briefs/a-stop-waits-for-the-unit-boundary-U1a-brief.md, section U1a.2) against the main that holds U1a.1; Codex builds it in a registered workspace, Opus reads, then the lane lands it (decision 14); U1b follows from the design's D10 section and BO-1. Fold the U1a.1 read's six non-material notes (lane-test-20261009/briefs/a-stop-waits-for-the-unit-boundary-U1a.1-read.md: N-1 no test pins the health prober; N-2 heldWorkCompleted passes a zero time to SeatAtUnitBoundary; N-3 two cases rejected by stage text not the terminal-step rule; N-4 kernel reads left at seat_start.go:697/705 and recovery_request.go:128/136; N-5 revival timestamps on wall time inside the runner; N-6 stewardCensusFor's optional prober) into U1a.2's brief.
- OpenedAt: 2026-10-20T13:32:22Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-20T13:32:22Z NN2YACSAQ4ZYQZM7AW8TZPW84Q-m1e-f2f350ed open actor=human:Wido targets=a-stop-waits-for-the-unit-boundary-2
Integrity: sha256=6085653b28c61f9e47ac8aa535082b77ff1b9eb79d9c22f1107ad877dbd0cdac
