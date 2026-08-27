# delegate-job-liveness

- State: claimed
- Intent: The live delegation lane comes back under metasystem custody, and delegated worker status must not lie. Evidence through 2026-08-27: at least six zombie/launch failures in two days — status fields raced or lied in four sightings 2026-08-26/27, then the correction pass for actionable-metrics double-fired at launch (a >10min first call retried, minting a duplicate resume whose id was handed back while the real task ran under another) and later zombied (status running 15+min after log, work products, and process were all dead; caught only by a coordinator's hand-rolled monitor applying the work-product-mtime + process-probe + timed-verdict triad). Root cause: the fleet's real lane (codex companion, raw codex exec) runs outside the job-record machinery entirely — no job record since 2026-08-12, no process identity, no idempotent launch, no cap, no reaper. Every protection exists in the metasystem dispatch lane and none of it applies to the lane in daily use (Wido approved this arc re-scope 2026-08-27)
- Origin: human
- Next step: HIGH PRIORITY, CLAIMED m1. POSITION ~21:15: satellite S1 (custody-launch-machine) round 1 = 11 material, ALL FOLDED into v2 (plans/custody-launch-machine-design.md) — the big moves: GENERATIONS LIVE IN THE TAG (per-reservation nonce in instanceTag; the wall-clock floor is deleted — no clock-step or same-second hazards), events-file-only progress stream (the child's own stdout; supervisor log excluded; watermark contract pinned for S2), the honest shared-checkout retreat (products are liveness evidence only in isolated workspaces; declared roots in shared checkouts report attribution only), tag-POSITION proof per the registry contract, full request fingerprint with canonicalization, named lock order with a bounded inside-lock phase, adoption totality incl. zero-leader and cross-PGID arms. S1 round 2 of 3 IN FLIGHT (failsafe r3; verdict at artifacts/agents/critiques/custody-launch-machine/r2-output.md, monitored). Parent map + rulings 1-6 unchanged. S2/S3 design notes follow S1's close. Appetites per satellite at close (RULING 4).
- OpenedAt: 2026-08-27T06:12:18Z
- Revision: 8
- Claimed: machine=m1 lineage=coordinator at=2026-08-27T14:09:04Z

History:
- 2026-08-27T06:12:18Z E8F1PERWGM23AZ2B7JPCWP1BS6-m1-bf243850 open actor=human:wido targets=delegate-job-liveness
- 2026-08-27T11:38:28Z MMQB7VDR1FJ4DT5MRQGQ9RAQ3F-m1-bf243850 edit actor=human:wido targets=delegate-job-liveness
- 2026-08-27T14:09:04Z KSDFQVTR5V1MCPHZQPQNT87V3K-m1-bf243850 claim actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-27T14:21:28Z 59MDW8SCZE9RAP43RH20VVYDFN-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-27T14:44:44Z BGTVS65BQ8TXWZBMAF9AE38Q3Z-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-27T15:19:07Z G70SXY7S1GEMQCRR2N7Y5GC2S1-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-27T17:06:51Z 060RZMB4E58GRGKGR8FWX2DTVZ-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-27T17:20:05Z MV5P20X85Z3DESKZ2TTT463547-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
Integrity: sha256=3363fb0ac2ea94269e4eb14440e33b1a3d768af1c16f11a21c77bd2535fb1cdd
