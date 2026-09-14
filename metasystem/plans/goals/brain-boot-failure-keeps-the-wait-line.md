# brain-boot-failure-keeps-the-wait-line

- State: approved
- Priority: 1
- Sequence: 58
- Risk: severity=3 novelty=1 exposure=3 accumulation=2 basis="severity 3: a revived seat whose brain boot fails or times out is armed but loses the durable line naming the job it waits on, so it may stop with work in flight; novelty 1: the same early-exit shape as session-start-relays-the-durable-wait-line; exposure 3: every seat start on a loaded machine; accumulation 2: silent, so repeats go unnoticed"
- Tier: 2
- Intent: Since 157fac92, a brain boot failure or timeout at SessionStart (scripts/agents/supervision-hook.sh near 725, five-second default deadline) exits through start_finish notice brain-boot or brain-timeout (near 731-772) before wait recovery. start_finish still arms supervision on that path, but session start never runs, so the seat gets no durable wait line. 157fac92^ added the brain failure as a notice and went on to up and session start. Found on 2026-09-15 by the Opus read of session-start-relays-the-durable-wait-line, which fixed only the identity-refused and arming-failed branches. DONE: a brain boot failure or timeout still publishes the durable wait line whenever session start succeeds; a fixture proves it under a forced brain timeout; plans/supervision-hook-root-design.md's notice-only brain outcome (near line 1680) says so; metasystem audit hook-start-exits requires the wait capture on those paths; no Stop decision changes.
- Origin: human
- Next step: Free. Builds on session-start-relays-the-durable-wait-line; claim after that lands.
- OpenedAt: 2026-09-14T22:42:13Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T22:42:20Z revision=2 opid=VXFZFFAFVGHPVFVSPXE3X50W96-m1e-c6925449 authority=proven digest=26a87807d2d49addd3822a24b1e49006feaa9ac2d9ca71b7654579ffd37386b7

History:
- 2026-09-14T22:42:13Z KR1744A7Y9D68TKM79TXY24HKT-m1e-c6925449 open actor=human:Wido targets=brain-boot-failure-keeps-the-wait-line reason=TierOverride: derived=3 set=2 why=Opened by m1c in Wido's name under R-110-m1e: a second SessionStart path that 157fac92 made drop the wait line, found by an independent read.
- 2026-09-14T22:42:20Z VXFZFFAFVGHPVFVSPXE3X50W96-m1e-c6925449 approve actor=human:Wido targets=brain-boot-failure-keeps-the-wait-line
- 2026-09-14T22:42:30Z G33GGB0N1DQSWJRE10JB0X6082-m1e-c6925449 set-priority actor=human:Wido targets=brain-boot-failure-keeps-the-wait-line reason=priority-order subject=brain-boot-failure-keeps-the-wait-line from=unranked to=1:59 requested-sequence=append
- 2026-09-14T23:04:04Z DA8WVH5YFWZ68JTBV0KVXPYVYB-m1e-c6925449 done actor=human:Wido targets=brain-boot-failure-keeps-the-wait-line,session-start-relays-the-durable-wait-line,suite-progress-bed-runs-on-trunk reason=priority-order from=1:59 to=1:58
Integrity: sha256=3ab2d5503b8fc10b336e95edabe24ef55ad5268ce299c7bba135569d6e492420
