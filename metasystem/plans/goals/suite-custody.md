# suite-custody

- State: queued
- Intent: Validation suites run under process-group custody: killing a suite reaps its whole tree, and gate locks carry pids and self-clean when their owner dies (2026-08-24 collateral: orphaned go-gate child blocked the next battery)
- Origin: human
- Next step: Appetite: 3h. battery.sh and validate-metasystem.sh become process-group owners: one pgid per run, EXIT/TERM traps reap the group, the gate-run guard records the owner pid and treats a dead owner as a stale lock to clear with a note instead of a refusal. Acceptance: kill -TERM a running battery mid-suite and the next battery starts clean with zero manual sweeps.
- OpenedAt: 2026-08-24T13:24:00Z
- Revision: 1

History:
- 2026-08-24T13:24:00Z BTXPEJND104017B02XP26P6Q2N-m2-bc1be9cb open actor=human:wido targets=suite-custody
Integrity: sha256=838c7f178ea6ffe7e7f23aa893503a2c28a47feb491a8c0250c7b59d61948bb9
