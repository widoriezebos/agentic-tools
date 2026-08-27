# delegate-job-liveness

- State: claimed
- Intent: The live delegation lane comes back under metasystem custody, and delegated worker status must not lie. Evidence through 2026-08-27: at least six zombie/launch failures in two days — status fields raced or lied in four sightings 2026-08-26/27, then the correction pass for actionable-metrics double-fired at launch (a >10min first call retried, minting a duplicate resume whose id was handed back while the real task ran under another) and later zombied (status running 15+min after log, work products, and process were all dead; caught only by a coordinator's hand-rolled monitor applying the work-product-mtime + process-probe + timed-verdict triad). Root cause: the fleet's real lane (codex companion, raw codex exec) runs outside the job-record machinery entirely — no job record since 2026-08-12, no process identity, no idempotent launch, no cap, no reaper. Every protection exists in the metasystem dispatch lane and none of it applies to the lane in daily use (Wido approved this arc re-scope 2026-08-27)
- Origin: human
- Next step: HIGH PRIORITY, CLAIMED m1. POSITION ~22:00: S1 (custody-launch-machine) round 2 = 11 material, FLAT vs round 1 — the loop-critiquing-itself signal noted in the record (about half the findings arose from round-1 folds). All 11 folded into v3: the PRE-FORK MARKER (advance-written evidence separating the recycled-pgid and pre-registration cases — the round's hardest epistemic hole), NONCE-GLOBAL adoption (the reservation-unique tag makes cross-group adoption sound; the cross-PGID incident rule retired), ordered/total R-C identity table, per-session occupancy index for an O(session) busy gate, waiter-loop wait bounds immune to clock steps, worktree root containment, versioned fingerprint encoding + dispatch-mode/resumed-session fields, MatchShape tag-position proof with the kill-path substring arm retired. Parent map carries the D-B inheritance refinement (single cap AUTHORITY, bounded hold). S1 ROUND 3 (FAILSAFE) IN FLIGHT — closes on all-fixture-expressible; if material findings survive it, exhaustion #1 is recorded and the fixtures-as-arbiter exit is weighed on the trajectory (mechanical-grain only). Verdict lands at artifacts/agents/critiques/custody-launch-machine/r3-output.md, monitored.
- OpenedAt: 2026-08-27T06:12:18Z
- Revision: 9
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
Integrity: sha256=59cbea9e049ab9b66ec09590ebf77fa40d574ba75f0cc6a7d27cbb23c316f34f
