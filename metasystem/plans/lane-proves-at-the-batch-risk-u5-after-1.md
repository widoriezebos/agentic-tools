# Unit U5 correction 1: canary groups do not decide cheapness; the fix record carries the check's wall time

Working Mode: Implement. Correction of unit U5 (brief `plans/lane-proves-at-the-batch-risk-u5-cheapness-measure.md`) after its Opus read. The worktree holds U5 uncommitted on origin/main b67642167; change only what this brief names.

## F-1 (material): every real plan reads "not cheap"

`test impact --plan` always selects the contract's canary groups (`test_impact.go` ~147: `policy-canary`, `adapter-canary`, `command-interface-smoke`); `command-interface-smoke`'s packages include `cmd/metasystem`, so `impactDepth` (~284, ~292) sets `selected["cmd/metasystem"]` for EVERY plan and returns not cheap: impact depth can never be chosen, every gate is skipped, every batch proves full (acceptance "a low-tier batch's proof under 25 minutes" and R-149-m1e "below it the change decides" cannot be met). Reproduced by the read on the real contract: two small packages alone 1% cheap; the same plus the three canaries 4% not cheap. Fix: groups selected regardless of the change (the contract's `Always.Canary` list) are left out of the cheapness measure on both sides (numerator and the cmd/metasystem rule) and out of the denominator's non-Go groups; only change-driven selections decide. Test with the REAL canary shape: a plan of two small packages plus the three canaries -> cheap, 1-2%; the pcc shape (named cmd tests from the change) -> not cheap (mutation: count canaries -> not cheap, test fails).

## F-2 (material, brief point 3): the fix record carries no wall time

`plain/fix.go` ~20-37 unchanged: the check's duration is on stdout and in `repair-check.json` only when saved green; a red or unsaved check (the 48-minute run that motivated U5) leaves its time on no record. Fix: the fix record gets the last check's wall time and result (`CheckMinutes`, `CheckResult`) written by the check path whatever the outcome; `landing status` shows it on the resolving/reviewing line. Test: a red check writes its minutes on the fix record (mutation: write only on green -> test fails).

## Also (not material, one line each if cheap)

Remove the now-dead `fullGroupCount` and `goadapter.TestPackageCount` if nothing but tests uses them (move the tests to the new functions); display the share floored, consistent with the threshold comparison.

## Checks

`go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestLandingDepth|TestLandingCheck|TestTestImpact|TestLandingGate|TestLandingProve|TestLandingImpact|TestLandingStatus'`; `./internal/landing/plain ./internal/landing/batch/goadapter`; `go run ./cmd/devgate static`; `./internal/audit ./internal/layering`. Never edit testing.json; never open metasystem.conf.local; do not commit. Report: diff --stat, each exit, the share/reason for: two small packages + canaries, the pcc shape, everything.
