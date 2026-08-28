# delegate-job-liveness

- State: claimed
- Intent: The live delegation lane comes back under metasystem custody, and delegated worker status must not lie. Evidence through 2026-08-27: at least six zombie/launch failures in two days — status fields raced or lied in four sightings 2026-08-26/27, then the correction pass for actionable-metrics double-fired at launch (a >10min first call retried, minting a duplicate resume whose id was handed back while the real task ran under another) and later zombied (status running 15+min after log, work products, and process were all dead; caught only by a coordinator's hand-rolled monitor applying the work-product-mtime + process-probe + timed-verdict triad). Root cause: the fleet's real lane (codex companion, raw codex exec) runs outside the job-record machinery entirely — no job record since 2026-08-12, no process identity, no idempotent launch, no cap, no reaper. Every protection exists in the metasystem dispatch lane and none of it applies to the lane in daily use (Wido approved this arc re-scope 2026-08-27)
- Origin: human
- Next step: HIGH PRIORITY, CLAIMED m1, CODE CRITIQUE ROUND 2 IN FLIGHT (custodial driver, monitored). POSITION 2026-08-28 ~04:35: round 1 = 8 material (canonical joinable findings; headline: the main dispatch lanes never called the state machine built for them). All eight corrected across SIX focused passes; each pass's own regressions caught and killed by the suites (reservation provenance, heartbeat fixture timing on bash 3.2 whole-second stamps, the critique call site, the worktree-vs-operational-state exclusion carve-out, the job-id-collision refusal keeping its name, fixture beds minting run-current capability snapshots after the canned one AGED PAST 30 DAYS AT MIDNIGHT — living proof of the Stale-checks metric thesis, follow-up session derived from the parent record). WHOLE TREE double-green under coordinator execution: all Go packages, dispatch-fixtures x2, adapter-deadline, metrics fixtures. BOUNDARY NOTE: two delegate-authored receipts rows removed (receipts are the coordinator's ledger; briefs now forbid it). Round 2 joins the folds + fresh pass; rounds 2-3 remain in budget; clean verdict → LAND (build + doctrine edit + records) with receipt goal=delegate-job-liveness built_by=delegate, then release for S2/S3 designs.
- OpenedAt: 2026-08-27T06:12:18Z
- Revision: 14
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
- 2026-08-27T20:09:37Z 1C17C8Z2W06Z3W2S2W4732G23A-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-28T00:30:39Z F9TKNCGGQT3JRWHEH8XGPQA5HQ-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-28T02:45:15Z 1AP5BV6VASX3J88336NCRDFKEV-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-28T04:42:46Z ARJHJFSQRG6H2C9PHNAY3B2TET-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
Integrity: sha256=1c4b7b7060ff2c4e630abd3f4974cac58284ae5e18206e063ee221f5a3e7efef
