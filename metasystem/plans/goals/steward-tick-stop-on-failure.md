# steward-tick-stop-on-failure

- State: claimed
- Intent: Residual from the twice-fixed steward tick flake: the landed version of TestRunLoopTicksUntilTheStopFile (4a5ef499, m2's line) writes the runner stop file only on the happy path, so an assertion failure leaks the live RunLoop goroutine into TempDir teardown — the exact race the flake registry diagnosed ('TempDir teardown race with the live tick goroutine'). m0 reproduced that leak class live: a failed iteration's loop printed 'tick failed: record tick completion: open /tmp/Test.../steward-tick.json: no such file or directory' into the NEXT iteration's output. m0's independently built fix (97336c30, preserved on branch machine/m0) closed it with a t.Cleanup handshake registered after goroutine launch: write stop file, drain the done channel with a 30s failsafe, ordered before TempDir removal by cleanup LIFO; a close(done) after RunLoop returns keeps the double-receive safe. Port that handshake onto the landed version; the patience half needs no change.
- Origin: main
- Next step: DONE, landed 65c36111 by m0b: the cleanup handshake from machine/m0's 97336c30 is ported onto the landed test — close(done) after the RunLoop send, t.Cleanup writes the stop file and drains the loop with a 30s failsafe before TempDir teardown. Chain implementer-0bc4adc1169d0aae26816254 (MECHANICAL, closed, Sol-built, conformance reviewedTree c503171a). Proof: 20/20 -race idle in the delegate; 160/160 green as 8 concurrent -race instances on the 4-CPU guest (the contention profile that exposed the defect), re-proven 5/5 after the landing rebase. Concluding is the opener's act.
- OpenedAt: 2026-08-31T19:08:58Z
- Revision: 5
- Budget: elapsedLimit=1d attemptLimit=3 reservedJobMinutesLimit=120 activeJobLimit=1
- Sliced: machine=m0b lineage=main-1788250419-3170380-8a1fb3 revision=3 at=2026-09-01T08:26:44Z
- Claimed: machine=m0b lineage=main-1788250419-3170380-8a1fb3 at=2026-09-01T08:24:21Z revision=3
- StopCapability: generation=3 revision=3 machine=m0b claimEpoch=1 fenceEpoch=0

History:
- 2026-08-31T19:08:58Z GNN3P2ZYCMZTKRHWPGVWPMGVGS-m0-c5dbf036 open actor=m0+main-1788178136-1684505-4ffe42 targets=steward-tick-stop-on-failure
- 2026-08-31T19:59:35Z 1Y5FJN7Q92K04NVSAGBTJDXSXG-m0-c5dbf036 set-budget actor=human:Wido targets=steward-tick-stop-on-failure
- 2026-09-01T08:24:21Z YXC5XV8G9YJDKE35JGV8CCZ3B3-m0b-6638932d claim actor=m0b+main-1788250419-3170380-8a1fb3 targets=steward-tick-stop-on-failure
- 2026-09-01T08:26:44Z 8V969SPSV7Y2T9M7KV405MY21G-m0b-6638932d slice-start actor=m0b+main-1788250419-3170380-8a1fb3 targets=steward-tick-stop-on-failure
- 2026-09-01T08:44:42Z BFR5RZCCM97ZM7H306FBMCFJEA-m0b-6638932d edit actor=m0b+main-1788250419-3170380-8a1fb3 targets=steward-tick-stop-on-failure
Integrity: sha256=ea096c3901b6c9eaee0fc40db32789ead2045f9d77bdd40323a8b0f14ef6e25b
