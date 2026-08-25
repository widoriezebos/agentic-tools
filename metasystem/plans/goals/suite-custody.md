# suite-custody

- State: claimed
- Intent: Validation suites run under process-group custody: killing a suite reaps its whole tree, and gate locks carry pids and self-clean when their owner dies (2026-08-24 collateral: orphaned go-gate child blocked the next battery)
- Origin: human
- Next step: Appetite: 3h. DE-CONFLICTED 2026-08-25 vs m1's gate-run-freeze (isolated-worktree batteries + verb freeze + stale-gate-lock hygiene live THERE): this item keeps the halves the freeze does not touch — (1) process-group custody: battery.sh / validate-metasystem.sh / supervision-fixtures.sh become process-group owners whose EXIT/TERM traps reap the whole tree, so killing a suite orphans nothing; (2) the KI-43 ancestry fix at the SUITE level: a suite leg that arms supervision must classify by an identity the suite CARRIES (become_main at suite entry for the harness root, or an equivalent announced identity), so detached launches (cron, steward revival, CI, nohup) run green — acceptance: a fully detached battery passes the S4 takeover leg. Delivery per Wido's 2026-08-25 discipline: coordinator design brief, codex-critiqued to convergence, BUILT BY CODEX, fresh-session critique to AGREE, fast tests only; no battery until the batch warrants it.
- OpenedAt: 2026-08-24T13:24:00Z
- Revision: 3
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-25T18:30:20Z

History:
- 2026-08-24T13:24:00Z BTXPEJND104017B02XP26P6Q2N-m2-bc1be9cb open actor=human:wido targets=suite-custody
- 2026-08-25T18:30:20Z X9X4M0GTTVZ3DNET65423FCY5Y-m2-bc1be9cb claim actor=m2+mac-coordinator targets=suite-custody
- 2026-08-25T18:31:17Z HNKG26A2NPMRBE0M8CX48509G7-m2-bc1be9cb edit actor=m2+mac-coordinator targets=suite-custody
Integrity: sha256=9afe1b3ae9f2311e9dad0fc0a67eb5d2e2c864eeff9fde1367414383365c0c48
