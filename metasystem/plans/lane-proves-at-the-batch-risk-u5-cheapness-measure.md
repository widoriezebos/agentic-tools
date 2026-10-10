# Unit U5 of lane-proves-at-the-batch-risk: the cheapness measure counts every selected package

Working Mode: Implement. Design: `plans/designs/lane-proves-at-the-batch-risk.md`, decisions D2 and D2b (the impact runs only when it is cheaper than the full). Size: at most 150 production lines. Smallest change.

## Measured defect (lane test finding 87)

`impactDepth` (`cmd/metasystem/test_impact.go` ~221-242) counts only WHOLE-package selections (groups whose tests are `"all"`) in the numerator; a selection of named tests in a package (the common shape for cmd/metasystem and for any package where the selector can name tests) counts nothing. The denominator (`fullGroupCount`) counts every contract group plus the expanded test-bearing packages. For goal process-changes-cover-declarations-and-interventions (three files in cmd/metasystem and internal/goal/branch) the rule said "cheap" and the resolution job's `metasystem test impact --check` ran 48 minutes (21:42-22:30), while the full proof takes 35-55 minutes; the gate (U3) would have repeated it. Also `repair-check.json` was not written after that check (verify why: the save is skipped when the tree changed during the check or files are unstaged; report what happened).

## Fix

1. Numerator: the number of test-bearing packages with ANY selected test, whole or named (a group whose tests are a named list counts its package once), plus contract groups selected outside the Go adapter. A selection that names any test of cmd/metasystem counts as cmd/metasystem and is never cheap (it is 63 percent of the suite's CPU time). Denominator: the full plan's test-bearing packages plus its non-Go groups (same definition as the numerator), computed once per decision.
2. Keep `landing.impact-max-share` (default 50) and the reason texts; the reason names the measure (`impact would cover 97% of 110 packages: full`).
3. Record on every impact check and gate the wall time it took next to the decision (the status line and the fix/gate record), so the measure can be judged against the full proof's time.
4. The `--check` save: when the check completes green, write `repair-check.json` whatever the working tree state EXCEPT when the staged tree hash changed during the check (then say so in the output); unstaged files alone never skip the save.

## Tests

- pcc-shaped plan (named cmd/metasystem tests + whole internal/goal/branch + dependents) -> not cheap (mutation: count whole only -> cheap, test fails).
- A plan of two small internal packages out of 110 -> cheap with share 2% (unchanged behaviour).
- The denominator excludes nothing the numerator counts (a plan selecting everything gives 100%).
- The check save writes with unstaged files present and skips only on a changed staged tree.
- Run every existing test touching impactDepth/impactCost/fullGroupCount/batchDepth and the check save (grep, by name): `go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestLandingDepth|TestLandingCheck|TestTestImpact|TestLandingGate|TestLandingProve|TestLandingImpact'`; `./internal/landing/plain ./internal/landing/batch/goadapter`; `go run ./cmd/devgate static`; `./internal/audit`.

Never edit testing.json; never open metasystem.conf.local; do not commit. Report: diff --stat, each exit, the share and reason for the pcc-shaped plan before/after.
