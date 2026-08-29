# delegate-job-liveness

- State: claimed
- Intent: The live delegation lane comes back under metasystem custody, and delegated worker status must not lie. Evidence through 2026-08-27: at least six zombie/launch failures in two days — status fields raced or lied in four sightings 2026-08-26/27, then the correction pass for actionable-metrics double-fired at launch (a >10min first call retried, minting a duplicate resume whose id was handed back while the real task ran under another) and later zombied (status running 15+min after log, work products, and process were all dead; caught only by a coordinator's hand-rolled monitor applying the work-product-mtime + process-probe + timed-verdict triad). Root cause: the fleet's real lane (codex companion, raw codex exec) runs outside the job-record machinery entirely — no job record since 2026-08-12, no process identity, no idempotent launch, no cap, no reaper. Every protection exists in the metasystem dispatch lane and none of it applies to the lane in daily use (Wido approved this arc re-scope 2026-08-27)
- Origin: human
- Next step: L12 LANDED 2637083 (+floor-key fix 39403b8): claim-launch CLI surface + call-site wiring live end to end - dispatch.sh fresh/follow paths on the claim state machine with m1's admission chain woven in v4 lock order; six new job verbs (claim-launch, claim-occupancy-prepare, prefork-mark, custody-groups, reconcile-reservation, ownership-patch) + proc group-owned; adapters carry output-stream/prefork/tag-match; janitor shapes + GroupOwnership; internal/progress evaluator (KEEP verdict) with RecordSetup scope capture; reap-facts reconciliationDue; RecordSetup unconditional reservation carry (fixture-proven). Dispatch/adapter/mission-runner bed GREEN end to end, all area race suites green. EXCLUDED per lane: delegate verb + custodial-exec routing (m1's L13; custodial fixture scenarios travel with it). Remaining goal scope (liveness verb semantics) = L13, m1's lane.
- OpenedAt: 2026-08-27T06:12:18Z
- Revision: 24
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=60 activeJobLimit=1
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-29T21:10:34Z revision=22
- StopCapability: generation=22 revision=22 machine=m2 claimEpoch=1 fenceEpoch=0

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
- 2026-08-28T05:41:55Z VVKVX853GSHBZW2YQF6RFR4ZJT-m1-bf243850 edit actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-28T05:47:56Z 3WRQ5RYNWS7Z3X7Z19DAA7C5ST-m1-bf243850 release actor=m1+coordinator targets=delegate-job-liveness
- 2026-08-29T17:45:42Z DYXCFN281R1R86C32RWQ7JS2RH-m2-bc1be9cb set-budget actor=human:wido targets=delegate-job-liveness
- 2026-08-29T17:45:56Z TSVCYXQB4NWQDJYDT1PAEPERS0-m2-bc1be9cb claim actor=m2+mac-coordinator targets=delegate-job-liveness
- 2026-08-29T18:26:42Z PG0C4Q1WH5VTNXKWKR2KNCWKWX-m2-bc1be9cb edit actor=m2+mac-coordinator targets=delegate-job-liveness
- 2026-08-29T18:26:56Z P81715TMG5XTCTZFPSNHDV10P0-m2-bc1be9cb release actor=m2+mac-coordinator targets=delegate-job-liveness
- 2026-08-29T21:10:20Z P4H9XS9BM60HWGJPB7KM0W2JYD-m2-bc1be9cb claim actor=m2+mac-coordinator targets=delegate-job-liveness
- 2026-08-29T21:10:34Z SC4QVPD8H67QK4CH1Q4DJZNNP6-m2-bc1be9cb set-budget actor=human:wido targets=delegate-job-liveness
- 2026-08-29T21:10:59Z 1HYX3GPTPXNZFSW6MYAZ39280E-m2-bc1be9cb edit actor=m2+mac-coordinator targets=delegate-job-liveness
- 2026-08-29T23:49:49Z 6B6HST95CF9YT1G2G8RWM61CF1-m2-bc1be9cb edit actor=m2+mac-coordinator targets=delegate-job-liveness
Integrity: sha256=206c7723a9658141e2cf7a5597370a1c63b44649aa1736f60681e0eae43cc707
