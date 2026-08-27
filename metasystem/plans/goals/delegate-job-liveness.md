# delegate-job-liveness

- State: claimed
- Intent: The live delegation lane comes back under metasystem custody, and delegated worker status must not lie. Evidence through 2026-08-27: at least six zombie/launch failures in two days — status fields raced or lied in four sightings 2026-08-26/27, then the correction pass for actionable-metrics double-fired at launch (a >10min first call retried, minting a duplicate resume whose id was handed back while the real task ran under another) and later zombied (status running 15+min after log, work products, and process were all dead; caught only by a coordinator's hand-rolled monitor applying the work-product-mtime + process-probe + timed-verdict triad). Root cause: the fleet's real lane (codex companion, raw codex exec) runs outside the job-record machinery entirely — no job record since 2026-08-12, no process identity, no idempotent launch, no cap, no reaper. Every protection exists in the metasystem dispatch lane and none of it applies to the lane in daily use (Wido approved this arc re-scope 2026-08-27)
- Origin: human
- Next step: HIGH PRIORITY, CLAIMED m1. POSITION ~19:45: design at v4 (plans/delegate-job-liveness-design.md), trajectory 13-10-7, FIRST BUDGET EXHAUSTED at the declared failsafe round 3 — exhaustion recorded with all seven open findings dispositioned (R3-01 RULED by Wido in-session: RULING 5 at-least-once stall-incident delivery; six folded: busy-session totality, product-root ownership at claim-launch, the seam verdict-to-status state machine with an archive entry rule, total timestamp semantics with event-time monotonic-max, reaper-vs-sweep dual death outcomes, the seam-handle reducer). FIVE Wido rulings now bind the design. Successor budget rounds 4-6 IN FLIGHT (round 4 dispatched, monitored; verdict at artifacts/agents/critiques/delegate-job-liveness/r4-output.md); early close on all-fixture-expressible; SECOND exhaustion stops the design on Wido. On close: per-slice appetites to Wido (RULING 4), then codex builds slice one. Side evidence: goal-git-stderr-pollution hit twice more by critics (sightings 3 and 4).
- OpenedAt: 2026-08-27T06:12:18Z
- Revision: 6
- Claimed: machine=m1 lineage=coordinator at=2026-08-27T14:09:04Z

History:
- 2026-08-27T06:12:18Z E8F1PERWGM23AZ2B7JPCWP1BS6-m1-bf243850 open actor=human:wido targets=delegate-job-liveness
- 2026-08-27T11:38:28Z MMQB7VDR1FJ4DT5MRQGQ9RAQ3F-m1-bf243850 edit actor=human:wido targets=delegate-job-liveness
- 2026-08-27T14:09:04Z KSDFQVTR5V1MCPHZQPQNT87V3K-m1-bf243850 claim actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-27T14:21:28Z 59MDW8SCZE9RAP43RH20VVYDFN-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-27T14:44:44Z BGTVS65BQ8TXWZBMAF9AE38Q3Z-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-27T15:19:07Z G70SXY7S1GEMQCRR2N7Y5GC2S1-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
Integrity: sha256=47a3713e588c468d94ebe2a4cb09f4e3669e1c45c7cf12b9b03a5a99882d45c6
