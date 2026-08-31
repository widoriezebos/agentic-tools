# steward-tick-load-flake

- State: claimed
- Intent: TestRunLoopTicksUntilTheStopFile flakes under load (wall-clock patience assumption) - m2's finding from the night of 2026-08-29, on the m1 steward seam
- Origin: main
- Next step: Appetite: 1h. Make the test patience load-tolerant the way the suite-custody work did elsewhere: condition-based waiting on the tick evidence, never wall-clock sleeps; prove by running the steward suite under artificial load
- OpenedAt: 2026-08-30T14:57:28Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=3 reservedJobMinutesLimit=90 activeJobLimit=1
- Claimed: machine=m3 lineage=mac-m3 at=2026-08-31T13:54:57Z revision=3
- StopCapability: generation=3 revision=3 machine=m3 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-30T14:57:28Z J8H2SS5TP3H03J240G5JGS3AMA-m1-bf243850 open actor=m1+coordinator targets=steward-tick-load-flake
- 2026-08-30T15:17:17Z AK0NK66HGBV8Q8RPQ4XB540QX6-m1-bf243850 set-budget actor=m1+coordinator targets=steward-tick-load-flake
- 2026-08-31T13:54:57Z 3XHTRA51333C230NRSFZVSYSFM-m3-a5da21ff claim actor=m3+mac-m3 targets=steward-tick-load-flake
Integrity: sha256=f9ec420aedc87147dae8bd501bdad4fb37f8bb54d99368dbc7eb8b69ba6d79da
