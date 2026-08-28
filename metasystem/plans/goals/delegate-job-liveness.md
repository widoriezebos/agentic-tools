# delegate-job-liveness

- State: claimed
- Intent: The live delegation lane comes back under metasystem custody, and delegated worker status must not lie. Evidence through 2026-08-27: at least six zombie/launch failures in two days — status fields raced or lied in four sightings 2026-08-26/27, then the correction pass for actionable-metrics double-fired at launch (a >10min first call retried, minting a duplicate resume whose id was handed back while the real task ran under another) and later zombied (status running 15+min after log, work products, and process were all dead; caught only by a coordinator's hand-rolled monitor applying the work-product-mtime + process-probe + timed-verdict triad). Root cause: the fleet's real lane (codex companion, raw codex exec) runs outside the job-record machinery entirely — no job record since 2026-08-12, no process identity, no idempotent launch, no cap, no reaper. Every protection exists in the metasystem dispatch lane and none of it applies to the lane in daily use (Wido approved this arc re-scope 2026-08-27)
- Origin: human
- Next step: HIGH PRIORITY, CLAIMED m1, BUILD COMPLETE — MANDATORY CODE CRITIQUE RUNNING THROUGH ITS OWN SUBJECT. POSITION 2026-08-28 ~02:50: ALL SIX STAGES double-green under coordinator execution (45 files, +2452/-524; seven correction passes total, each evidence-driven — the census candidate-universe narrowing landed in two steps: foreign-EPERM exclusion then the 60s age slack per the nonce-cannot-predate-its-minting law). LANE PROVEN LIVE: supervision re-armed (dead Aug-16 owner replaced via --rearm after sweeping leaked fixture holds; arming-dead-owner-takeover 1h backlogged for the misleading failure), session announced as main, lease held — and the custodial critique driver completed a real end-to-end round against live codex: the FIRST production custodial job record since Aug 12 (proven identity, terminal-stamped, schema-validated return). CODE-CRITIQUE ROUND 1 of 3 IN FLIGHT on chain custody-launch-machine-code THROUGH the custodial lane itself (--role code-critic, sol/xhigh, monitored; canonical findings-array returns enable mechanical joins). On the verdict: adjudicate, correct, iterate to clean, then LAND (custody build + never-stop doctrine edit + records) with receipt goal=delegate-job-liveness built_by=delegate. After landing: parent S2 (sweep) and S3 (seam) designs; metrics slices un-gate.
- OpenedAt: 2026-08-27T06:12:18Z
- Revision: 13
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
Integrity: sha256=22f64d8053b0675eb81c1dc6b2b40555d9a4424cc591c1797b805002501fb6e7
