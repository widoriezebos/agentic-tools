# steward-tick-stop-on-failure

- State: queued
- Intent: Residual from the twice-fixed steward tick flake: the landed version of TestRunLoopTicksUntilTheStopFile (4a5ef499, m2's line) writes the runner stop file only on the happy path, so an assertion failure leaks the live RunLoop goroutine into TempDir teardown — the exact race the flake registry diagnosed ('TempDir teardown race with the live tick goroutine'). m0 reproduced that leak class live: a failed iteration's loop printed 'tick failed: record tick completion: open /tmp/Test.../steward-tick.json: no such file or directory' into the NEXT iteration's output. m0's independently built fix (97336c30, preserved on branch machine/m0) closed it with a t.Cleanup handshake registered after goroutine launch: write stop file, drain the done channel with a 30s failsafe, ordered before TempDir removal by cleanup LIFO; a close(done) after RunLoop returns keeps the double-receive safe. Port that handshake onto the landed version; the patience half needs no change.
- Origin: main
- Next step: One small slice: add the cleanup handshake from machine/m0's version to the landed test; prove with -race -count=20 under 8x CPU load (the profile that exposed both defects). Under 4h, robustness gain (R-33)
- OpenedAt: 2026-08-31T19:08:58Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=3 reservedJobMinutesLimit=120 activeJobLimit=1

History:
- 2026-08-31T19:08:58Z GNN3P2ZYCMZTKRHWPGVWPMGVGS-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=steward-tick-stop-on-failure
- 2026-08-31T19:59:35Z 1Y5FJN7Q92K04NVSAGBTJDXSXG-m0-c5dbf036 set-budget actor=human:Wido targets=steward-tick-stop-on-failure
Integrity: sha256=8b77d665b3c461a373979c787199ae112e92009c96b1b0c88e63a0a9f4f4b3fc
