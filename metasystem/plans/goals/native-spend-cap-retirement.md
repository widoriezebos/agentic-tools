# native-spend-cap-retirement

- State: claimed
- Intent: KILL THE PER-WORKER DOLLAR CAP (Wido's order 2026-09-01, verbatim: 'backlog item to be executed immediately: KILL THIS STUPID CAP. But... as you have learned the hard way: backlog, design, critique. build, critique build. NO EXCEPTION. Make sure we do not inflict self-harm using this. The assumption is that we have enough protection in the machinery already. The assumption is that this one is a stupid one that actually harms us. If these are true (enough) then proceed and kill this stupid idea'): the engine's hardcoded $5.00 default --max-budget-usd on every Claude delegate (internal/adapter/claude.go ClaudeBudget, landed 24345044 on 2026-08-14 inside an argv consolidation, never designed or critiqued) fires only AFTER spend, cannot distinguish runaway from expensive legitimate work, and killed nine finished-but-unreported workers across three machines (three on m1/m2's ledger-attention chain, six on m0b's alert-channel design day 2026-09-01, each costing a paid recovery round plus 40 pool-minutes). CONDITIONAL MANDATE: the design must VERIFY both of Wido's stated assumptions before the kill — (1) enumerate the existing spend-bounding machinery (reserved job-minutes, per-round cap-minutes with watchdog, turn limit, attempt limits, breach fences, stop-loss conduct) and show each runaway scenario has a surviving owner; (2) establish the harm from the recorded specimens. If both hold (enough), the cap dies; if a real unowned runaway scenario emerges, the design says so and proposes the minimal owner instead
- Origin: main
- Next step: Ladder from the top, no exception: Fable design round (small, focused - the adapter code and protection inventory, not the big alert doc) then Sol design critique then Sol build then Fable code critique then tests. Related: budget-death-on-return (the specimens), dispatch-cap-necessity (the reservation-unit sibling)
- OpenedAt: 2026-09-01T16:58:42Z
- Revision: 6
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=480 activeJobLimit=1
- Sliced: machine=m0b lineage=main-1788250419-3170380-8a1fb3 revision=3 at=2026-09-01T16:59:53Z
- Claimed: machine=m0b lineage=main-1788250419-3170380-8a1fb3 at=2026-09-01T19:12:26Z revision=6
- StopCapability: generation=6 revision=6 machine=m0b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-01T16:58:42Z Z9D8J1HM0KPD26ZH925X2SRTM7-m0b-6638932d open actor=m0b+main-1788250419-3170380-8a1fb3 targets=native-spend-cap-retirement
- 2026-09-01T16:58:46Z MYKMR55TB45G5XZXS7FYFVPX5Y-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=native-spend-cap-retirement
- 2026-09-01T16:59:11Z ZW39353JGM7F65TZNG8W5MNYRB-m0b-6638932d claim actor=m0b+main-1788250419-3170380-8a1fb3 targets=native-spend-cap-retirement
- 2026-09-01T16:59:53Z E42WEB8PKDTEGHJ8WQV78V0MD4-m0b-6638932d slice-start actor=m0b+main-1788250419-3170380-8a1fb3 targets=native-spend-cap-retirement
- 2026-09-01T17:55:48Z M8R70XSYZZ35Y6J6DMVZEGAXEV-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=native-spend-cap-retirement
- 2026-09-01T19:12:26Z N1A8NPS419QJ25W3H40ZW5NM0M-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=native-spend-cap-retirement
Integrity: sha256=cdbb32a8ff632d3d4be81ead89859eb805285ebdb5af3da93806e90184975fa3
