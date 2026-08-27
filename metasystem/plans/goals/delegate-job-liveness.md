# delegate-job-liveness

- State: claimed
- Intent: The live delegation lane comes back under metasystem custody, and delegated worker status must not lie. Evidence through 2026-08-27: at least six zombie/launch failures in two days — status fields raced or lied in four sightings 2026-08-26/27, then the correction pass for actionable-metrics double-fired at launch (a >10min first call retried, minting a duplicate resume whose id was handed back while the real task ran under another) and later zombied (status running 15+min after log, work products, and process were all dead; caught only by a coordinator's hand-rolled monitor applying the work-product-mtime + process-probe + timed-verdict triad). Root cause: the fleet's real lane (codex companion, raw codex exec) runs outside the job-record machinery entirely — no job record since 2026-08-12, no process identity, no idempotent launch, no cap, no reaper. Every protection exists in the metasystem dispatch lane and none of it applies to the lane in daily use (Wido approved this arc re-scope 2026-08-27)
- Origin: human
- Next step: HIGH PRIORITY, CLAIMED m1, BUILD RUNNING CONTINUOUSLY (Wido's never-stop order 2026-08-27 ~23:15 — canonical doctrine landed in docs/orchestration.md pending-commit; the morning timer was deleted and the build started immediately; never-idle-enforcement 2h opened for the mechanical encoding). POSITION ~00:15: STAGE 1 (identity core) BUILT — failing-first honored, focused Go green under coordinator execution — but MY integration run caught TWO defects codex's sandbox couldn't see: (1) the happy dispatch leg reproducibly hits the 40s start-identity ceiling → launch_failed with null identity (suspect: the new kernel-probing exact identity bypasses the fixture fake-identity seam, or the started-at emission/CAS contract broke the shell parser); (2) dispatch-fixtures.sh EXITED 0 over that red leg — silent-green honesty bug. CORRECTION PASS IN FLIGHT (direct codex exec, monitored; brief demands diagnosis-with-proof, both-worlds fix real+fixture, double green run, and the suite exiting non-zero on the broken leg). Then stages 2-6 per plans/custody-launch-machine-build-brief.md, code-critique chain, land. Doc edit to orchestration.md rides the next clean landing window.
- OpenedAt: 2026-08-27T06:12:18Z
- Revision: 11
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
Integrity: sha256=6de0ae7bb29d57b7f0cd7b1ce1b211ce6826d109f7f769a22eb3d45027e1dd65
