# steward-tick-load-flake

- State: queued
- Intent: TestRunLoopTicksUntilTheStopFile flakes under load (wall-clock patience assumption) - m2's finding from the night of 2026-08-29, on the m1 steward seam
- Origin: main
- Next step: DONE, landed by m0 (account Wido@M0): the test now follows the patience doctrine — progress-resetting 30s failsafe on the tick wait, generous stop failsafe, and a cleanup handshake that stops and drains the RunLoop goroutine on every exit path before TempDir teardown (the registry's diagnosed leak). Reproduced 2-in-20 red under -race + 8x load pre-fix; 20/20 green after in delegate worktree and orchestrator environment. Chain steward-tick-patience-handshake closed; job record honestly carries a process-lost supervision verdict (the delegate's own load evidence starved the 4-CPU guest — a real lesson for load-generating evidence commands on small machines, noted in the receipt).
- OpenedAt: 2026-08-30T14:57:28Z
- Revision: 5
- Budget: elapsedLimit=3d attemptLimit=4 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-08-30T14:57:28Z J8H2SS5TP3H03J240G5JGS3AMA-m1-bf243850 open actor=m1+coordinator targets=steward-tick-load-flake
- 2026-08-30T15:17:17Z AK0NK66HGBV8Q8RPQ4XB540QX6-m1-bf243850 set-budget actor=m1+coordinator targets=steward-tick-load-flake
- 2026-08-31T13:09:26Z ZCQ173VB6W6WMEDEQ68PKK4H25-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=steward-tick-load-flake
- 2026-08-31T13:45:32Z NSH5333VWYMH4ZSZY4GDWYHE6Q-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=steward-tick-load-flake
- 2026-08-31T13:45:34Z STKH11BNS2N2EXPKW58J4PNSYX-m0-c5dbf036 release actor=m0+main-1788178136-1684505-4ffe42 targets=steward-tick-load-flake
Integrity: sha256=d749b70999589d9054f754c24c33d210cb4a85c7c3a4761f5bb5d0d3ba11b95d
