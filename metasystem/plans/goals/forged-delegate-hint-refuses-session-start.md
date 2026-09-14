# forged-delegate-hint-refuses-session-start

- State: queued
- Risk: severity=3 novelty=1 exposure=3 accumulation=2 basis="severity 3: a SessionStart carrying a forged delegate hint now exits 0 where it must refuse, and the supervision-and-census section stops before any scenario runs; novelty 1: a side effect of c71f1e23's continuation path; exposure 3: every landing whose proof selects that section; accumulation 2: the stopped section hides every later scenario's result"
- Tier: 2
- Intent: Since c71f1e23 (session-start-relays-the-durable-wait-line, landed by m1c), section/supervision-and-census-fixtures is red on trunk and stops before any supervision scenario runs. runtime-hook-fixtures.sh:384 ('a forged delegate hint suppressed a fresh host') runs supervision-hook.sh claude start with METASYSTEM_HOOK_DELEGATE_JOB=job-forged, whose job record has pid 1 and pidStartedAt 1, and expects a non-zero exit; the hook now exits 0. Reported by m1e on 2026-09-15 from harness-tracked runs on fad3ace8 (tree 07464224) and on its C1a candidate (tree 2925a10f); the line is absent from every earlier run of the section, and only c71f1e23 touched the hook in that window. m1c's verification of c71f1e23 ran the supervision-hook and supervision beds but not this section's runtime-hook prelude. DONE: the cause is recorded; a forged delegate hint at SessionStart refuses as before; c71f1e23's wait-line guarantee holds (wait-restart, TestWaitSessionStart*); every section that testing.json selects for scripts/agents/supervision-hook.sh passes on the candidate; metasystem audit hook-start-exits passes; no Stop decision changes.
- Origin: human
- Next step: Codex Sol diagnosis and fix dispatched by m1c in scratchpad g12/wt22; m1c claims it at landing.
- OpenedAt: 2026-09-14T23:42:58Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-14T23:42:58Z GWXJMTMRHG77P56F4VPBR6Z9XX-m1e-c6925449 open actor=human:Wido targets=forged-delegate-hint-refuses-session-start reason=TierOverride: derived=3 set=2 why=Opened by m1c in Wido's name under R-110-m1e: a trunk regression from m1c's c71f1e23, reported by m1e.
Integrity: sha256=df83036e1491eed5f8082a7189716d673f8a1e79945c0684f45df377368b4985
