# the-lane-session-owns-its-merge

- State: approved
- Risk: severity=3 novelty=1 exposure=3 accumulation=2 basis="The lane's merge resolver installed a dirty engine; every lane session since failed arming silently and no merge resolution could run for a day."
- Tier: 3
- Intent: A check never replaces an enrolled checkout's engine (one publication policy at every devgate install boundary), a session whose arming fails asks the person once at the source with the remedy and retires the question when it arms, and a lane stop closes once main moves past its tree; root cause of the lane's silent merge refusals (finding 612).
- Origin: main
- Next step: Build U1 (60), U2 (98), U3 (32, --last; may land first) from /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/the-lane-session-owns-its-merge.r4.md (Astra DESIGN STANDS r4). After seat-engines-follow-the-deployed-engine lands: deploy, P4 bootstrap of the lane, verify announce, lease succession and merge dispatch (page Remediation).
- OpenedAt: 2026-11-12T09:09:13Z
- Revision: 2
- Budget: elapsedLimit=2d attemptLimit=12 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=1T7DXRZ8AY6AZ725SSJZMB6NAH-m1e-bfec86c9 minutes=2400 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-11-12T09:12:24Z revision=2 opid=1T7DXRZ8AY6AZ725SSJZMB6NAH-m1e-bfec86c9 authority=proven digest=131cdeae7dad7514ab7ba6cc4f8784f98aefe4a5f5240c5eb55c92133cdd3044 episode=2

History:
- 2026-11-12T09:09:13Z 26H7XRTBZXTGN0FHTSJVS0RRSA-m1e-bfec86c9 open actor=human:Wido targets=the-lane-session-owns-its-merge
- 2026-11-12T09:12:24Z 1T7DXRZ8AY6AZ725SSJZMB6NAH-m1e-bfec86c9 approve actor=human:Wido targets=the-lane-session-owns-its-merge
Integrity: sha256=f454242d4f6c4138e37a79f4fdd3e727ad904fbf761a265e28f50218d7db515f
