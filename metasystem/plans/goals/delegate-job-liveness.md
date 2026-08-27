# delegate-job-liveness

- State: claimed
- Intent: The live delegation lane comes back under metasystem custody, and delegated worker status must not lie. Evidence through 2026-08-27: at least six zombie/launch failures in two days — status fields raced or lied in four sightings 2026-08-26/27, then the correction pass for actionable-metrics double-fired at launch (a >10min first call retried, minting a duplicate resume whose id was handed back while the real task ran under another) and later zombied (status running 15+min after log, work products, and process were all dead; caught only by a coordinator's hand-rolled monitor applying the work-product-mtime + process-probe + timed-verdict triad). Root cause: the fleet's real lane (codex companion, raw codex exec) runs outside the job-record machinery entirely — no job record since 2026-08-12, no process identity, no idempotent launch, no cap, no reaper. Every protection exists in the metasystem dispatch lane and none of it applies to the lane in daily use (Wido approved this arc re-scope 2026-08-27)
- Origin: human
- Next step: HIGH PRIORITY, CLAIMED m1. POSITION ~20:30: ROUND 4 rose to 12 material (9 shape-level; trajectory 13-10-7-12) — the no-progress signal Wido's doctrine names, applied at the FIRST rise. WIDO RULED THE SPLIT (RULING 6) + AMENDED RULING 3 (seam records to a SIBLING registry; canonical consumers untouched — the lease-succession breakage round 4 found is thereby avoided, not migrated). The parent design is now the MAP: rulings 1-6, converged ground, and round-4 finding routing to three ordered satellites — S1 custody-launch-machine (R4-01/02/07/08/12; design v1 WRITTEN at plans/custody-launch-machine-design.md: presence-vs-progress signal classes with the artificial log-toucher removed, fingerprinted total claim-launch with reservation deadlines and atomic busy+reserve under the extended cap-authority lock, identity read sandwich + exact-token tags, shared-checkout product law with a fixed exclusion set, generation-floor adoption/death/occupancy); S2 sweep+episodes (R4-05/09/10/11); S3 seam domain on the sibling store (R4-03/04/06). S1 critique ROUND 1 of 3 IN FLIGHT (chain custody-launch-machine, failsafe r3, monitored; verdict at artifacts/agents/critiques/custody-launch-machine/r1-output.md). S2/S3 design notes follow S1's close, in order. Appetites per satellite at each close (RULING 4).
- OpenedAt: 2026-08-27T06:12:18Z
- Revision: 7
- Claimed: machine=m1 lineage=coordinator at=2026-08-27T14:09:04Z

History:
- 2026-08-27T06:12:18Z E8F1PERWGM23AZ2B7JPCWP1BS6-m1-bf243850 open actor=human:wido targets=delegate-job-liveness
- 2026-08-27T11:38:28Z MMQB7VDR1FJ4DT5MRQGQ9RAQ3F-m1-bf243850 edit actor=human:wido targets=delegate-job-liveness
- 2026-08-27T14:09:04Z KSDFQVTR5V1MCPHZQPQNT87V3K-m1-bf243850 claim actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-27T14:21:28Z 59MDW8SCZE9RAP43RH20VVYDFN-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-27T14:44:44Z BGTVS65BQ8TXWZBMAF9AE38Q3Z-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-27T15:19:07Z G70SXY7S1GEMQCRR2N7Y5GC2S1-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-27T17:06:51Z 060RZMB4E58GRGKGR8FWX2DTVZ-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
Integrity: sha256=b1c8b9f635df5bddec07a364252d24fe0e0bbdf746439556908446b5a9cfdaac
