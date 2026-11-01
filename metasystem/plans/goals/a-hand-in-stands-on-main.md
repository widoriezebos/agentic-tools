# a-hand-in-stands-on-main

- State: approved
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="engine code on the hand-in path and the landing merge; a wrong rebase breaks a goal branch (recoverable from origin); every hand-in is exposed; it recurs whenever main moves under a goal"
- Tier: 2
- Intent: A goal is handed in on main: before work land the engine brings the goal branch onto main tip (rebase or merge of main), runs the unit check there and reads that merge at the seat, so the lane proves only trees that already stand on main and a semantic conflict is found at the seat, not after a full lane cycle and a return (a-round-survives-a-restart was returned 2026-11-01 07:22 for a conflict with a Thursday commit in internal/lease/sweep.go that merged without text conflict; finding 464 context)
- Origin: human
- Next step: Design through the engine: metasystem design write a-hand-in-stands-on-main --brief /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/a-hand-in-stands-on-main-request.md; then design review; then units
- OpenedAt: 2026-11-01T06:39:06Z
- Revision: 2
- Budget: elapsedLimit=5d attemptLimit=24 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=EFEHA56G9A5YV5EQQAC3RGGDPP-m1e-b2c714f5 minutes=2400 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-11-01T06:39:15Z revision=2 opid=EFEHA56G9A5YV5EQQAC3RGGDPP-m1e-b2c714f5 authority=proven digest=fae1cf8f5f7af721302738e3d948489a9a7d07d03e5526787ebddd5ed3af2e77 episode=2

History:
- 2026-11-01T06:39:06Z 84C0KGPTF0M9T5FT0MBYRB96ZP-m1e-b2c714f5 open actor=human:Wido targets=a-hand-in-stands-on-main
- 2026-11-01T06:39:15Z EFEHA56G9A5YV5EQQAC3RGGDPP-m1e-b2c714f5 approve actor=human:Wido targets=a-hand-in-stands-on-main,a-red-is-classified-once
Integrity: sha256=3e7df2a0daf8a14355152e1db3c36db9a8b1595817a00115088a9c28a73fc9cd
