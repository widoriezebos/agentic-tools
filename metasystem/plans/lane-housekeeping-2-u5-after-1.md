# lane-housekeeping-2 unit 5 correction 1: a repeated trunk clear is success, not a refusal (R-129-ui)

Working Mode: Implement. Correction of the one-transaction unit after its Opus read (RETURN, 1 material). Worktree uncommitted; change only what this brief names; leave fixes uncommitted; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json.

## F-1 (material, R-129-ui): the batch clear refuses when the same trunk result is cleared twice
`internal/goal/trunkred.go` ~834-867: the batch Mutate skips only AlreadyApplied; an entry already Closed under a different opid returns TRUNK_RED_CLOSED and rejects the WHOLE batch, so a second clear of the same attempt (a classify of a saved red twice gives the same NextFull proof; a push clearing from a stale register after a records-only fold) fails with "main passed its full check, but its incidents could not be cleared". Fix: in the batch Mutate, an entry whose Closed is already set for the same attempt (Closed.Attempt == args.Attempt) counts as satisfied like AlreadyApplied; a fully satisfied batch answers already-applied and writes nothing (no second record, R-129). Test: a second ULID clears the same attempt → success, no commit (real-origin test: rev-list count unchanged); mutation: drop the satisfied rule → TRUNK_RED_CLOSED → test fails.

## Folds (non-material, cheap)
- Messages: "1 incident" / "N incidents" (grammar); a legacy single clear names its entry id and never prints an empty attempt; the cadence-claim record does not print "record : 0 incidents" (name the cadence claim instead).
- `TestTrunkRedRealRegisterRoundTrips` reads only the frozen fixture, not the live ledger page.

## Checks
`go vet ./internal/goal ./cmd/metasystem`; `go test -count=1 -timeout 30m ./internal/goal ./internal/layering`; `go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestIncident|TestTrunk|TestLanding|TestAudit'`; `go run ./cmd/devgate static`. Report: diff --stat, every exit, the mutation.
