# supervision-fixture-stop-hook-evidence

- State: queued
- Tier: 1
- Intent: The supervision fixture suite's stop-hook-monitor scenario (scripts/agents/supervision-fixtures.sh) now passes its block-once and health-line assertions but fails the last one, 'the stop hook left no evidence that it ran', because artifacts/agents/supervision/hooks.log under the scenario's stop root is empty or absent after the stop-hook fix (6e0221e0) changed how the hook records itself. Seen seat-side on m2 2026-09-04. DONE means the assertion reads the evidence the current hook writes, or, if the hook writes none, the hook writes it again; never a deleted assertion.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one assertion or one hook line): build, run supervision-fixtures.sh seat-side, land through a chain; box 1h/3/60m/1. Waits for human approval for execution; Wido 2026-09-04: 'land what you can, leave the rest on the backlog'.
- OpenedAt: 2026-09-04T13:14:06Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0

History:
- 2026-09-04T13:14:06Z 9FDG2Q32TXFHMHX7JWPTD703WX-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=supervision-fixture-stop-hook-evidence
Integrity: sha256=22977372fe701c4426e16f86dd71b0ff520d817c428486fe376510482e0a63ba
