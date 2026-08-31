# steward-tick-load-flake

- State: done
- Intent: TestRunLoopTicksUntilTheStopFile flakes under load (wall-clock patience assumption) - m2's finding from the night of 2026-08-29, on the m1 steward seam
- Origin: main
- Next step: Appetite: 1h. Make the test patience load-tolerant the way the suite-custody work did elsewhere: condition-based waiting on the tick evidence, never wall-clock sleeps; prove by running the steward suite under artificial load
- Concluded: Landed 4a5ef499. TestRunLoopTicksUntilTheStopFile now uses progress-based stall patience (10s without evidence change) under one absolute test-deadline fail-stop, replacing the fixed 5s and 3s wall-clock waits that flaked three times in three days under machine load. Chain implementer-4c362d582c73b9ae9b1f63a9 fully conformance-reviewed and closed. Custodian proof outside the sandbox: focused x20 green, x5 green under 12 CPU-hog loops. Unblocks m2's final governed weight-discharge attempt.
- OpenedAt: 2026-08-30T14:57:28Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=3 reservedJobMinutesLimit=90 activeJobLimit=1

History:
- 2026-08-30T14:57:28Z J8H2SS5TP3H03J240G5JGS3AMA-m1-bf243850 open actor=m1+coordinator targets=steward-tick-load-flake
- 2026-08-30T15:17:17Z AK0NK66HGBV8Q8RPQ4XB540QX6-m1-bf243850 set-budget actor=m1+coordinator targets=steward-tick-load-flake
- 2026-08-31T13:54:57Z 3XHTRA51333C230NRSFZVSYSFM-m3-a5da21ff claim actor=m3+mac-m3 targets=steward-tick-load-flake
- 2026-08-31T14:13:02Z SC0CW12Y65JDQ65RNB9C6R6PBS-m3-a5da21ff done actor=m3+mac-m3 targets=steward-tick-load-flake
Integrity: sha256=6d62f029c4bc364b67be172acc4e1450390fdba34af21ad9e3b119e66df297d8
