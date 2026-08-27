# delegate-job-liveness

- State: claimed
- Intent: The live delegation lane comes back under metasystem custody, and delegated worker status must not lie. Evidence through 2026-08-27: at least six zombie/launch failures in two days — status fields raced or lied in four sightings 2026-08-26/27, then the correction pass for actionable-metrics double-fired at launch (a >10min first call retried, minting a duplicate resume whose id was handed back while the real task ran under another) and later zombied (status running 15+min after log, work products, and process were all dead; caught only by a coordinator's hand-rolled monitor applying the work-product-mtime + process-probe + timed-verdict triad). Root cause: the fleet's real lane (codex companion, raw codex exec) runs outside the job-record machinery entirely — no job record since 2026-08-12, no process identity, no idempotent launch, no cap, no reaper. Every protection exists in the metasystem dispatch lane and none of it applies to the lane in daily use (Wido approved this arc re-scope 2026-08-27)
- Origin: human
- Next step: HIGH PRIORITY, CLAIMED m1. POSITION ~18:30: ROUND 1 = 13 material + 2 not (no early close — five findings were decision-shaped). WIDO RULED ALL FOUR DECISIONS in-session: (1) LANE A ONLY — the custodial channel is the sole sanctioned launch lane, the companion demoted to observation records (proofLevel seam, self-report never terminals a record); (2) SESSION-WIDE REFUSAL on busy sessions; (3) THIS ARC OWNS the one canonical jobs/ registry with immutable proofLevel (session-coexistence re-points to consume at design close); (4) APPETITE PAUSED — re-appetite at convergence, critique-driver cutover built once with the severity task. Design v2 folds all 13 (claim-launch state machine won/in-progress/bound/refused with three-way recycle-proof identity incl. persisted ticks/bootID; survivors join over recorded custody so supervisor-only death never terminal-stamps; complete D-F liveness decision table incl. seam-handle rot; steward-owned sweep with first-tick-past-ceiling SLA, goal-free record scan, durable stalledSuspectedAt). Round 2 of 3 IN FLIGHT (failsafe r3; verdict at artifacts/agents/critiques/delegate-job-liveness/r2-output.md, monitored). On the verdict: adjudicate, fold, close or round 3; then per-slice appetites to Wido.
- OpenedAt: 2026-08-27T06:12:18Z
- Revision: 5
- Claimed: machine=m1 lineage=coordinator at=2026-08-27T14:09:04Z

History:
- 2026-08-27T06:12:18Z E8F1PERWGM23AZ2B7JPCWP1BS6-m1-bf243850 open actor=human:wido targets=delegate-job-liveness
- 2026-08-27T11:38:28Z MMQB7VDR1FJ4DT5MRQGQ9RAQ3F-m1-bf243850 edit actor=human:wido targets=delegate-job-liveness
- 2026-08-27T14:09:04Z KSDFQVTR5V1MCPHZQPQNT87V3K-m1-bf243850 claim actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-27T14:21:28Z 59MDW8SCZE9RAP43RH20VVYDFN-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-27T14:44:44Z BGTVS65BQ8TXWZBMAF9AE38Q3Z-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
Integrity: sha256=2f53b3f178372261ab60dc16865a41b4030f49152315dbca848963bcc9834c93
