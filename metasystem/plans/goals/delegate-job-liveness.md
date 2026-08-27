# delegate-job-liveness

- State: claimed
- Intent: The live delegation lane comes back under metasystem custody, and delegated worker status must not lie. Evidence through 2026-08-27: at least six zombie/launch failures in two days — status fields raced or lied in four sightings 2026-08-26/27, then the correction pass for actionable-metrics double-fired at launch (a >10min first call retried, minting a duplicate resume whose id was handed back while the real task ran under another) and later zombied (status running 15+min after log, work products, and process were all dead; caught only by a coordinator's hand-rolled monitor applying the work-product-mtime + process-probe + timed-verdict triad). Root cause: the fleet's real lane (codex companion, raw codex exec) runs outside the job-record machinery entirely — no job record since 2026-08-12, no process identity, no idempotent launch, no cap, no reaper. Every protection exists in the metasystem dispatch lane and none of it applies to the lane in daily use (Wido approved this arc re-scope 2026-08-27)
- Origin: human
- Next step: HIGH PRIORITY (Wido 2026-08-27), CLAIMED m1. POSITION ~17:15: fact pass DONE — 83 anchored facts at plans/delegate-job-liveness-facts.md (key: the ACP-seam wait is over, acp-adapter-seam DONE all slices; suite-dispatch-exclusion DONE 95e432d this morning — custodial launches ride its guard; the companion is uncustodiable from outside — no liveness check exists, worker pid never the CLI's, null at terminal, THE zombie mechanism confirmed in source; the steward ladder already detects untracked codex as VerdictUnknown — the gap is attribution+suppression; nothing today measures work-product mtime, the leg that caught every real zombie). DESIGN v1 at plans/delegate-job-liveness-design.md: lane-honest custody (Lane A full custody via the F38 minimal subset reusing existing verbs; Lane B seam custody via shadow records + the triad), idempotent launch by the txn pattern (job claim-launch verb), steward sweep with attribution, probe-resume-first, NO new kill authority, session-coexistence boundary flagged (D-G), 8h/three slices proposed. Critique ROUND 1 of 3 IN FLIGHT on chain delegate-job-liveness (sol/xhigh, failsafe r3; verdict at artifacts/agents/critiques/delegate-job-liveness/r1-output.md, monitored). On the verdict: adjudicate with dispositions in the design record, fold, iterate to close, then build slices on Wido-confirmed appetites.
- OpenedAt: 2026-08-27T06:12:18Z
- Revision: 4
- Claimed: machine=m1 lineage=coordinator at=2026-08-27T14:09:04Z

History:
- 2026-08-27T06:12:18Z E8F1PERWGM23AZ2B7JPCWP1BS6-m1-bf243850 open actor=human:wido targets=delegate-job-liveness
- 2026-08-27T11:38:28Z MMQB7VDR1FJ4DT5MRQGQ9RAQ3F-m1-bf243850 edit actor=human:wido targets=delegate-job-liveness
- 2026-08-27T14:09:04Z KSDFQVTR5V1MCPHZQPQNT87V3K-m1-bf243850 claim actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-27T14:21:28Z 59MDW8SCZE9RAP43RH20VVYDFN-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
Integrity: sha256=fd0d9cd149aef628f6c4978db1e1212aba86fa9139cc901fd91b8fc0c40cf229
