# lease-fold-empty-scan

- State: queued
- Intent: internal/lease/sweep.go groupOwnsTag is a second, independent group-ownership fold still carrying the pre-dab1dbd defect: an empty member scan returns provably-not-owned (owned=false, provable=true) - zero observations read as disproof; its consumer internal/run/conclude.go SweepStale then raises a false ownership-disproven and aborts the whole stale-run sweep for a group mid-reap. DONE means the lease fold matches the janitor fold's law: an empty scan proves nothing
- Origin: main
- Next step: Appetite: 1h. Found 2026-08-31 by the kill-guard-fold-consumers enumeration (m3). INTENT: align the lease fold's empty-scan semantics with the janitor fold (dab1dbd's law) and verify both its consumers (stopStaleGroup skip path, SweepStale surfacing path). CONSTRAINT: fail-closed stays - an unprovable scan must not become silent ownership either. Prove with the lease sweep and refusals test families
- OpenedAt: 2026-08-31T18:45:41Z
- Revision: 1

History:
- 2026-08-31T18:45:41Z VVCPPGC21C0XDKJ2BK0CS111A1-m3-a5da21ff open actor=m3+mac-m3 targets=lease-fold-empty-scan
Integrity: sha256=f8f6a76ff85a3748710b647cad662f8fe0a8d40c93cfc145ccb54bb5ec6e50e9
