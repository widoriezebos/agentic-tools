# Unit U2 of lane-proves-at-the-batch-risk: the batch's depth, chosen by its riskiest member

Working Mode: Implement. Design: `plans/designs/lane-proves-at-the-batch-risk.md`, decisions D1, D2, D2b and the settings. Builds on U1 (the `impact` scope exists). Size: at most 250 production lines.

## Build

1. Settings in `internal/config/defaults.go` with help text: `landing.full-from-tier` (default `3`), `landing.impact-max-share` (default `50`, percent).
2. `landing prove` with no scope flag on a batch tree decides the depth: read each batch member's goal tier (the goal file's tier); when the highest tier is at or above `landing.full-from-tier` -> `full`; else compute the impact plan (`metasystem test impact --plan --base <batch base>`) and the full plan's group count (the contract's groups the full would run); when the plan's whole-package groups reach `landing.impact-max-share` percent of the full's, or include `unit/cmd/metasystem` whole, -> `full` with reason `impact would cover N%: full`; else `impact`. The reason (`tiers 2,2; 41% of the tree` / `tier 3: full`) is recorded on the attempt and printed by `landing status` (`proving at impact depth: ...`). `--trunk` is unchanged (always full).
3. The push admits a green at the decided depth (today only a full green pushes: find the admission in push.go/prove.go and extend it to `impact` with the recorded reason).
4. D2b: the checks run by the resolution job (`work build ... --check 'metasystem test impact'`, skill case 4) and the lane's fix round use the same rule through one function: when impact is not cheap by rule 2, the check runs the `fast-static-build` group only and records `check: fast-static-build only; impact would cover N%`. The collection step (the repeated `work build` that collects the job) reuses the job's recorded check result when the staged tree hash and base are unchanged, recording `check reused from the job`, instead of running the check again (test: a collection on an unchanged staged tree runs no check; a changed tree runs it; mutation: always re-run).

## Tests

- Tier at/above the threshold -> full, reason names the tier; below -> impact, reason names the tiers (mutation: invert the comparison).
- Share above the threshold or cmd/metasystem whole -> full with the share reason; below -> impact (mutation: drop the cmd rule).
- Push admitted after an impact green (mutation: require full -> refused).
- The check rule: a not-cheap plan runs fast-static-build only and records the reason (mutation: always run impact).
- Status text for both depths.

Then `go run ./cmd/devgate static`, `go test -count=1 -timeout 30m ./internal/landing/plain ./internal/config`, every existing test using a changed seam (grep, by name), cmd tests by name (`TestLandingProve|TestLandingPush|TestLandingStatus|TestLandingResolve|TestLaneFix|TestSettings`). Never edit testing.json; never open metasystem.conf.local. Report: diff --stat, each exit, the two reason texts.
