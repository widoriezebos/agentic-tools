# a-hand-in-stands-on-main

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="engine code on the hand-in path and the landing merge; a wrong rebase breaks a goal branch (recoverable from origin); every hand-in is exposed; it recurs whenever main moves under a goal"
- Tier: 2
- Intent: A goal is handed in on main: before work land the engine brings the goal branch onto main tip (rebase or merge of main), runs the unit check there and reads that merge at the seat, so the lane proves only trees that already stand on main and a semantic conflict is found at the seat, not after a full lane cycle and a return (a-round-survives-a-restart was returned 2026-11-01 07:22 for a conflict with a Thursday commit in internal/lease/sweep.go that merged without text conflict; finding 464 context)
- Origin: human
- Next step: Design through the engine: metasystem design write a-hand-in-stands-on-main --brief /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/a-hand-in-stands-on-main-request.md; then design review; then units
- OpenedAt: 2026-11-01T06:39:06Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-01T06:39:06Z 84C0KGPTF0M9T5FT0MBYRB96ZP-m1e-b2c714f5 open actor=human:Wido targets=a-hand-in-stands-on-main
Integrity: sha256=c925e3b816786db8375f87f02c3576a22f108208f56cfd26bc3b8ccc312a2c79
