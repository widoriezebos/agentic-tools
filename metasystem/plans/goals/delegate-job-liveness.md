# delegate-job-liveness

- State: claimed
- Intent: The live delegation lane comes back under metasystem custody, and delegated worker status must not lie. Evidence through 2026-08-27: at least six zombie/launch failures in two days — status fields raced or lied in four sightings 2026-08-26/27, then the correction pass for actionable-metrics double-fired at launch (a >10min first call retried, minting a duplicate resume whose id was handed back while the real task ran under another) and later zombied (status running 15+min after log, work products, and process were all dead; caught only by a coordinator's hand-rolled monitor applying the work-product-mtime + process-probe + timed-verdict triad). Root cause: the fleet's real lane (codex companion, raw codex exec) runs outside the job-record machinery entirely — no job record since 2026-08-12, no process identity, no idempotent launch, no cap, no reaper. Every protection exists in the metasystem dispatch lane and none of it applies to the lane in daily use (Wido approved this arc re-scope 2026-08-27)
- Origin: human
- Next step: HIGH PRIORITY, CLAIMED m1. POSITION 2026-08-27 ~23:00 (day closed): S1 failsafe = 11 material, FLAT 11-11-11, critic-declared stop-loss; WIDO RULED THE IMPLEMENTATION-FIRST EXIT (RULING 7, per D81 — two prose budgets exhausted across the arc's core). ALL RECORDS LANDED (parent map with seven rulings, the 83-fact sheet as amended, S1 design v3 frozen with three rounds of dispositions, and the RULED BUILD BRIEF at plans/custody-launch-machine-build-brief.md: six-stage build order, every S1R3 finding a failing-first fixture, mandatory code-critique as arbiter, ~10h timebox). BUILD STARTS 2026-08-28 MORNING — the m1 coordinator session holds an armed 07:23 wakeup; if a FRESH session picks this up instead: execute the build brief verbatim (direct codex exec only, companion benched, zombie-triad monitors), verify by own execution, code-critique to clean, land via land.sh (lineage exported) + receipt with goal= built_by=. After S1's build: S2 (sweep/episodes) and S3 (seam domain, sibling registry) design notes per the parent map; then metrics slices two/three un-gate. Queue also holds steward-marks-retired-ledger (1h) and goal-git-stderr-pollution (1h, four critic sightings today).
- OpenedAt: 2026-08-27T06:12:18Z
- Revision: 10
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
- 2026-08-27T17:33:14Z 9K0CGB546AZBQ0D9VAH9GG2RY9-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-27T18:10:14Z 3R5QQG01ME1A0Z3GE26YEKM8NG-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
Integrity: sha256=c38833c7eeaf04db7dbaacb5a88f54786fd65dea19570f12b7555a98c22948a7
