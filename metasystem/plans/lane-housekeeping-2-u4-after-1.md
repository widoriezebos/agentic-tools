# lane-housekeeping-2 unit 4 correction 1: the check record counts every selected group; the declared check stays the project's

Working Mode: Implement. Correction after the Opus read (RETURN, 1 material + one note Wido's rules make material). Worktree uncommitted; change only what this brief names; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json. Checks sequential with -p 4 (or -p 2 under load).

## F-1 (material): the record leaves out the always-run contract groups
cmd/metasystem/test_impact.go ~543 impactSelectionForm skips every group without a `units` entry, but the contract groups added at ~192 (policy-canary, adapter-canary, command-interface-smoke — 264 named tests in cmd/metasystem) do run; every cheap `work commit` runs ~273 canary tests that the record does not name ("impact: 1 tests of 1 packages"). Fix: count every selected group — named tests or Go test functions and their packages, including the contract groups — or name the groups next to the counts ("impact: N tests of M packages + groups policy-canary, adapter-canary, command-interface-smoke"); the cheap-mode assertion in TestWorkCommitUnitImpactGitAdapter includes the canary (mutation: skip the groups → the assertion fails).

## N-2 (material by Wido's 2026-09-28 rule "machinery is language-generic"): work commit must not hard-code the Go impact check
`work commit` no longer reads the project's declared `proof.cheap` and hard-codes goadapter.UnitImpact / `test impact --check`. Fix: read the declared `proof.cheap` as before; when it is the metasystem's own impact verb (`metasystem test impact`, with or without flags) run it through the unit-impact form with `--check` and the cheapness decision (this repository's case, today's behaviour kept); otherwise run the declared command as declared (another language's project keeps its check). Test: a fixture declaring `proof.cheap=./check.sh` runs that script and records "declared: ./check.sh" (mutation: force the Go check → the fixture's script never runs → test fails).

## Folds (non-material)
- N-3: use testpolicy.GoTests instead of a second go/parser count if it is a drop-in; otherwise leave and say so.
- N-6: TestWorkCommitRealLaneFixImpactPlanGitAdapter pins the exact count 1411; assert "named tests of cmd/metasystem, fewer than the package's total" instead of the number.
- Record N-1 in the report as a follow-up (rebase/unit-review re-checks still run the full selection through carrySubjectCheck); do not build it here.

## Checks
`go vet ./cmd/metasystem ./internal/landing/plain`; `go test -p 4 -count=1 -timeout 30m ./internal/landing/plain ./internal/landing/batch/... ./internal/layering`; `go test -p 4 -count=1 -timeout 30m ./cmd/metasystem -run 'TestWorkCommit|TestTestImpact|TestLandingCheck|TestAudit'`; `go run ./cmd/devgate static`. Report: diff --stat, production line count (≤250 total), every exit, each mutation.
