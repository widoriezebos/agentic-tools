# proof-run-cost-and-liveness

- State: claimed
- Intent: A proof run can never again be silent, unbounded, and quadratically priced: pre-landing validation proves the STAGED BATCH once and nested runs reuse it, every suite announces its expected cost up front, emits progress heartbeats, and dies loudly at a silence failsafe instead of stalling dark (Wido 2026-08-27 evening: very serious bug, must never be allowed to happen again — 2h dirty-tree adopt run, 50min operator silence)
- Origin: main
- Next step: DESIGN RATIFIED (Wido 2026-08-27 late evening): all seven failsafe disputes resolved as recommended — FROZEN EXPORT (gate runs against a private tmp export of the manifest bytes; freezing, execution locus, and the A-B-A race resolved together); insider fabrication recorded as accepted single-user-trust risk (no new privilege boundary); kill inventory = supervision registry + execution-guard members + process group; Part 0 hoists ONLY the static grep placeholder scan; printing-but-wedged sections die at 3x their declared duration cap; evidence copy bounded (timeout + size cap, partial noted, kill unconditional). Design plans/proof-run-cost-and-liveness-design.md is the binding spec. BUILD SLICES: 1 (frozen-export witness + manifest digest + controller binding, 4h), 2 (progress contract: banner, JSONL heartbeat, sibling watchdog with bounded evidence + kill union, 4h), 3 (Part 0 hoist + structural section assertion + runner surfaces, 2h). Wido's mechanical-prevention condition is the acceptance bar throughout. Queue: immediately after memory-architecture slice 1 lands (in flight).
- OpenedAt: 2026-08-27T17:07:53Z
- Revision: 5
- Labels: shared
- Budget: elapsedLimit=2d4h attemptLimit=20 reservedJobMinutesLimit=700 activeJobLimit=2
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-28T22:51:22Z revision=5

History:
- 2026-08-27T17:07:53Z 15PY3WX8E8D98882HN5W21B1ND-m2-bc1be9cb open actor=human:wido targets=proof-run-cost-and-liveness
- 2026-08-27T17:22:10Z RQJ353PZ87KVYTCBFWFTX7Q2CP-m2-bc1be9cb edit actor=human:wido targets=proof-run-cost-and-liveness
- 2026-08-28T14:43:26Z MY4D0DRPYBP49F25N36FA0RGGB-m2-bc1be9cb edit actor=human:wido targets=proof-run-cost-and-liveness
- 2026-08-28T21:08:56Z EV7A4T81R4F1FAY251VP2N63G9-m2-bc1be9cb set-budget actor=human:wido targets=proof-run-cost-and-liveness
- 2026-08-28T22:51:22Z 2MAJNKGJ8K6JW2H7XN5M2YEE8R-m2-bc1be9cb claim actor=m2+mac-coordinator targets=proof-run-cost-and-liveness
Integrity: sha256=adf7c0820beea623485a7dc7ea4ffee8fe83e28d70d1c8ac46cc5974c93e785f
