# delegate-job-liveness

- State: queued
- Intent: The live delegation lane comes back under metasystem custody, and delegated worker status must not lie. Evidence through 2026-08-27: at least six zombie/launch failures in two days — status fields raced or lied in four sightings 2026-08-26/27, then the correction pass for actionable-metrics double-fired at launch (a >10min first call retried, minting a duplicate resume whose id was handed back while the real task ran under another) and later zombied (status running 15+min after log, work products, and process were all dead; caught only by a coordinator's hand-rolled monitor applying the work-product-mtime + process-probe + timed-verdict triad). Root cause: the fleet's real lane (codex companion, raw codex exec) runs outside the job-record machinery entirely — no job record since 2026-08-12, no process identity, no idempotent launch, no cap, no reaper. Every protection exists in the metasystem dispatch lane and none of it applies to the lane in daily use (Wido approved this arc re-scope 2026-08-27)
- Origin: human
- Next step: Appetite: 4h — HARD SLICE CEILING PER WIDO 2026-08-28 (no slice ever exceeds 4h again); this appetite governs WHATEVER continues on this goal and its breach banner must stay armed (the previous NextStep edits destroyed the parseable prefix and silenced appetite protection for the whole overnight build — the coordinator's own defect). STATE: ALL WORK STOPPED under Wido's stop order. The overnight build (6 stages + ~10 correction passes, UNLANDED) is preserved untouched on branch wip/custody-launch-machine (3fec78a, pushed). Steward/narrator were dead since Aug 20/23 (runner pid 18121 dead; being re-armed). NOTHING RESUMES without Wido's explicit word.
- OpenedAt: 2026-08-27T06:12:18Z
- Revision: 17
- Budget: elapsedLimit=2d attemptLimit=20 reservedJobMinutesLimit=400 activeJobLimit=2

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
Integrity: sha256=4ae1a37d3349ad80f70c38dccc902b6ea36cf8ffae0d62e0ff6c9e6ad9dc3118
