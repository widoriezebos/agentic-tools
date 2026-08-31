# lease-fold-empty-scan

- State: done
- Intent: internal/lease/sweep.go groupOwnsTag is a second, independent group-ownership fold still carrying the pre-dab1dbd defect: an empty member scan returns provably-not-owned (owned=false, provable=true) - zero observations read as disproof; its consumer internal/run/conclude.go SweepStale then raises a false ownership-disproven and aborts the whole stale-run sweep for a group mid-reap. DONE means the lease fold matches the janitor fold's law: an empty scan proves nothing
- Origin: main
- Next step: Appetite: 1h. Found 2026-08-31 by the kill-guard-fold-consumers enumeration (m3). INTENT: align the lease fold's empty-scan semantics with the janitor fold (dab1dbd's law) and verify both its consumers (stopStaleGroup skip path, SweepStale surfacing path). CONSTRAINT: fail-closed stays - an unprovable scan must not become silent ownership either. Prove with the lease sweep and refusals test families
- Concluded: Landed 3ba27a82. The lease sweep's ownership fold now matches the janitor law: an empty member scan proves nothing (owned=false, provable=false), the stale-run sweep leaves the unprovable run untouched, continues sweeping later runs, and surfaces the first unprovable scan with a distinct honest reason instead of a false ownership-disproven abort. Both consumers verified with new test rows; lease and run packages green in the orchestrator environment. Chain implementer-60e620c55e44c2c664301f61 conformance-reviewed and closed. Requested by m2 as pre-flight for the governed weight-discharge rerun.
- OpenedAt: 2026-08-31T18:45:41Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=3 reservedJobMinutesLimit=90 activeJobLimit=1

History:
- 2026-08-31T18:45:41Z VVCPPGC21C0XDKJ2BK0CS111A1-m3-a5da21ff open actor=m3+mac-m3 targets=lease-fold-empty-scan
- 2026-08-31T19:27:31Z P2MS814N1JKP50651MXYTZ8D04-m3-a5da21ff claim actor=m3+mac-m3 targets=lease-fold-empty-scan
- 2026-08-31T19:58:51Z EEKGCQ6F2G3AJGENP3SERB91N5-m3-a5da21ff done actor=m3+mac-m3 targets=lease-fold-empty-scan
Integrity: sha256=fe1ed8dbc6f7a81ee44d5a111bbb43e1e9c4798e3380df1602cb11cec9632064
