# lane-speed U2 correction 1: a named replay falls back to the whole unit when it is incomplete or inconclusive

Working Mode: Implement. Correction after the Opus read (RETURN, 2 material). Worktree uncommitted; change only what this brief names; leave fixes uncommitted; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json. The design copy in this worktree is now the current one (LS-03 folded into S2).

## F-1 (high): a test name missing on the replayed tree loses the attribution
Today's real red (attempt 20261011T111952.309251000Z): the failing test exists only on the goal's commit, not on main; the named replay on main's prefix records the missing identity under an empty unit ("(did not report)"), report.go ~72 rejects the LANDING-FAILED line with an empty unit, the replay reads "did not complete" and the cause stays unclassified — where the whole-unit replay was green on main and attributed `own` correctly. Fix (both): (1) in the reporter (internal/repoproof/full.go ~232), a requested name the package does not have is DROPPED, never failed; with no names left, the compile alone runs and reports complete; (2) in internal/landing/plain/replay.go, when a named replay is incomplete, or a recorded name is not a plain test identifier (a `Name(did not report)` token, a non-go group), the prefix falls back to the whole-unit replay. The classification record names which form ran and why ("named replay incomplete: whole unit").

## F-2 (medium, LS-03): a named replay green on main is INCONCLUSIVE
When the named replay is green on a prefix while the full run failed there (order-dependent failures), the prefix is re-run with the whole unit before attribution; only the whole-unit result attributes. Shares F-1's fallback branch. The record says "named replay green on main: whole-unit replay ran".

## Tests
Real shape: the red of attempt 20261011T111952… with its test absent on the base → named replay drops the name, runs compile only, result complete, attribution `own` as today (mutation: fail the missing name → unclassified → test fails); a `Name(did not report)` token → whole-unit fallback; a named green on main → whole-unit replay runs and attributes; fix `TestReporterReplaysRealRedAgainstTheRepositoryContract` so it does not pass when nothing ran.

## Folds (non-material)
`landing prove` help names LANDING_ONLY_TESTS; the record says which unit when whole; a single non-go group says "whole unit", never "named".

## Checks
`go vet ./internal/landing/plain ./internal/repoproof ./proof`; `go test -count=1 -timeout 30m ./internal/landing/plain ./proof ./internal/repoproof ./internal/layering`; `go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestLanding|TestClassify|TestAudit|TestReplay|TestARedProof'`; `go run ./cmd/devgate static` (the design-ledger complaint is expected). Report: diff --stat, production line count (≤250), every exit, each mutation.
