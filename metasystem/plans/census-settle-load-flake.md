# Supervision-fixture census-settle load flake (KI-37 residual)

STATUS: **ATTEMPTED, REVERTED, DEFERRED** (2026-08-19). The runtime-extension
approach below was implemented, critiqued (two rounds), unit-proven in
isolation, then REVERTED after guest integration testing showed it (a) makes
`scripts/validate-metasystem.sh` exit 2 under load where the baseline passes,
and (b) can inflate suite runtime to ~10 min per stuck census wait. Not landed.
The durable operator record remains KI-37 in `plans/known-issues.md`
(workaround: re-run on a quiescent guest).

## Symptom
Under heavy guest load, `scripts/agents/supervision-fixtures.sh` S4-6 settle
waits (e.g. `S4-6 unreadable start time` → verdict `CENSUS-FAILED` with a
`start-time-unreadable:` error) intermittently time out and fail
`scripts/validate-metasystem.sh`, blocking VM commits. Separate from and
downstream of the btime-seconds bug fixed in cc9a48f (issue #1 sweep 3).

## Root cause
The per-wait ceiling (`fixture_ceiling_sec`) is derived ONCE at harness start
from a single calibration census probe (`fixture-budget.sh`, floor 8× /
ceiling 48×). If the box is quiet at that instant but loaded later, the
background census loop keeps scanning but each scan is slow, so a converging
settle can outlast the one-shot cap although nothing is hung.

## Attempted fix (reverted)
Made the census-output waits progress-aware and opt-in (`--census`):
`wait_until`, at a `--census` wait's deadline, granted a bounded extension
(max 5) when `last-census.json`'s monotonic `completedAtEpoch` had advanced
since the window began. Isolation tests were clean (extend-then-pass; frozen
→ cap; legacy wait not extended by a live census; advancing-but-broken → 5
bounded then fail). Two adversarial critiques (codex gpt-5.6-sol MED, Claude
LOW) drove the global→opt-in scoping.

## Why it was reverted — guest integration findings
1. **Regression under load.** Baseline `supervision-fixtures.sh` passes
   `validate` (EXIT=0); with the change `validate` exits 2. The change passes
   STANDALONE (EXIT=0, S4-1..S4-16) but fails inside `validate`'s context.
   Root cause of the exit-2 not isolated (an ERR-trap probe was drowned by the
   `dispatch_fails` negative-test `set +e` blocks); would need several
   expensive `validate` runs (5–20 min each, load-sensitive) to pin down.
2. **Runtime blow-up.** Each extension re-arms a FULL scaled ceiling
   (base 12s × floor 8 = 96s), so 5 extensions = ~10 min per genuinely-stuck
   census wait. Across 11 tagged waits this can balloon the suite far beyond
   the flake it removes — arguably worse operationally.

## If revisited, a bounded redesign
- Extend by a SMALL FIXED increment (e.g. +20–30s), not a full re-armed
  ceiling; cap total added time (e.g. ≤90s) so the worst case is bounded and
  small.
- OR skip runtime extension entirely: fold current loadavg into the ONE-SHOT
  calibration (`fixture-budget.sh`) so an already-loaded box gets a wider cap
  upfront — zero change to per-wait runtime behavior, cannot cause the exit-2,
  but does not catch load that arrives AFTER calibration.
- Either way: first ROOT-CAUSE the exit-2 (is it the change, or a pre-existing
  dispatch-fixture load flake the longer runtime merely exposes?) before
  landing.
