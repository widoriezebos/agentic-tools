# suite-custody

- State: queued
- Intent: Validation suites run under process-group custody: killing a suite reaps its whole tree, and gate locks carry pids and self-clean when their owner dies (2026-08-24 collateral: orphaned go-gate child blocked the next battery)
- Origin: human
- Next step: LAYERS ONE AND TWO LANDED 50d7e6d (codex-built, conformance-green, certified; attended S4-1..16 green): the census identity override dies with its leg, the stop-hook leg announces the suite shell when no runtime ancestor exists, both suites reap on INT/TERM, and the announce scanner guards non-scratch call sites. RESIDUE (appetite spent at the declared line): a fully detached run now travels past BOTH previously-red layers and ends SILENTLY mid-suite around the unwatched-run stop-hook leg — no refusal, no green marker (evidence: session scratchpad supfix-final-proof.log, 2026-08-25). The next claimant starts at that leg with an xtrace; each peeled layer so far was one ambient-identity assumption, and the acceptance (fully detached green suite) still stands. Related: steward-owned-execution (detached execution is its world), KI-43 (mechanism section now diagnosed and landed).
- OpenedAt: 2026-08-24T13:24:00Z
- Revision: 5

History:
- 2026-08-24T13:24:00Z BTXPEJND104017B02XP26P6Q2N-m2-bc1be9cb open actor=human:wido targets=suite-custody
- 2026-08-25T18:30:20Z X9X4M0GTTVZ3DNET65423FCY5Y-m2-bc1be9cb claim actor=m2+mac-coordinator targets=suite-custody
- 2026-08-25T18:31:17Z HNKG26A2NPMRBE0M8CX48509G7-m2-bc1be9cb edit actor=m2+mac-coordinator targets=suite-custody
- 2026-08-25T20:37:49Z MYR0WR78CKJCBA44FK4G0Q2H18-m2-bc1be9cb edit actor=m2+mac-coordinator targets=suite-custody
- 2026-08-25T20:38:00Z N4007V77HVS7GE7XVD19QS0Z59-m2-bc1be9cb release actor=m2+mac-coordinator targets=suite-custody
Integrity: sha256=86b764afe014e58d1db4bfb146899c84fd424f19377c8e29f86ee5d51eada191
