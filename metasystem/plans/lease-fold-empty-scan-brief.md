Working Mode: implement
Orchestrator Identity: m3 (lineage mac-m3; DRAFT prepared 2026-08-31 during a machine-hold window — dispatch waits for the goal's claim)
Date: 2026-08-31

# Goal

The lease sweep's group-ownership fold stops treating an empty member
scan as proof of non-ownership. internal/lease/sweep.go groupOwnsTag
(lines ~227-257) is an independent copy of the ownership fold that
still returns (owned=false, provable=true) for an empty scan — zero
observations read as disproof, the exact reasoning error the janitor
fold abolished (dab1dbd: an empty scan proves nothing, INDETERMINATE).
Its consumer internal/run/conclude.go SweepStale (~409-415) then
raises "stale run group ownership disproven; surfacing" and aborts the
whole stale-run sweep for a group that may simply be mid-reap.

# Workspace

The dispatch-created job worktree, branched from main. May touch
EXACTLY (inside the metasystem/ project):

- internal/lease/sweep.go
- internal/lease/sweep_test.go
- internal/lease/refusals_test.go
- internal/run/conclude.go — ONLY if the new unprovable outcome needs
  distinct handling there; report a gap if the right handling is not
  mechanically determined by the rules below.

# Mechanical rules — no judgment calls

1. An empty scan yields UNPROVABLE, not disproof: groupOwnsTag returns
   (owned=false, provable=false) for zero live observations, matching
   the janitor fold's "empty scan proves nothing" law
   (internal/janitor/killproof.go:175-200 and its test rows).
2. Fail-closed stands both ways: unprovable never becomes silent
   ownership, and never becomes "disproven". Consumers:
   - stopStaleGroup (sweep.go ~191): an unprovable scan skips the stop
     exactly as not-owned does today (no signal on no proof) — verify,
     and pin with a test.
   - SweepStale (conclude.go ~409-415): an unprovable scan must NOT
     raise "ownership disproven"; it surfaces as its own reason naming
     the unprovable scan, and does NOT abort the remaining sweep loop
     (one unprovable group must not stop the sweep of the others) —
     if the surrounding code structure makes continue-on-surface
     ambiguous, stop and report the gap.
3. Tests: extend TestGroupOwnsTag (sweep_test.go:45) and
   TestGroupOwnsTagUnprovableRows (refusals_test.go:272) with the
   empty-scan row asserting provable=false; add the SweepStale
   continue-past-unprovable case; TestSweepFailsClosed
   (refusals_test.go:213) stays green unmodified.

# Constraints

- No changes to the janitor fold or any kill-guard surface.
- No test weakened. Wall-clock budget: 25 minutes.

# Expected Return

Version-2 implementer JSON; diffBoundary lists the touched files WITH
the metasystem/ prefix; evidence includes
`go test ./internal/lease/ -count=1` and, if conclude.go changed,
`go test ./internal/run/ -count=1` (report sandbox-denied runs as
environment-limited, never as green).

# Gap Rule

stop and report a gap; never fill it silently.
