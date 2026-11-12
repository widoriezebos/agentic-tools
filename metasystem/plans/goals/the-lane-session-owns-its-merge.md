# the-lane-session-owns-its-merge

- State: queued
- Risk: severity=3 novelty=1 exposure=3 accumulation=2 basis="The lane's merge resolver installed a dirty engine; every lane session since failed arming silently and no merge resolution could run for a day."
- Tier: 3
- Intent: A check never replaces an enrolled checkout's engine (one publication policy at every devgate install boundary), a session whose arming fails asks the person once at the source with the remedy and retires the question when it arms, and a lane stop closes once main moves past its tree; root cause of the lane's silent merge refusals (finding 612).
- Origin: main
- Next step: Build U1 (60), U2 (98), U3 (32, --last; may land first) from /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/the-lane-session-owns-its-merge.r4.md (Astra DESIGN STANDS r4). After seat-engines-follow-the-deployed-engine lands: deploy, P4 bootstrap of the lane, verify announce, lease succession and merge dispatch (page Remediation).
- OpenedAt: 2026-11-12T09:09:13Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-12T09:09:13Z 26H7XRTBZXTGN0FHTSJVS0RRSA-m1e-bfec86c9 open actor=human:Wido targets=the-lane-session-owns-its-merge
Integrity: sha256=7c11e726bb5cd60316bc3247d6ed03a85b2748f64aed275e47983c5597aad7d1
