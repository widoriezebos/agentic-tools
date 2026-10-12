# lane-batching U4 correction 2: the new verbs carry their idempotency rows and coverage examples

Working Mode: Implement. Correction after the FULL cmd/metasystem gate of the combined hot-fix (2026-10-12 05:25) failed two repository audits on this unit's verbs. Worktree uncommitted on top of the unit's commit a6bc57414 (branch build/lb4); leave fixes uncommitted; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json. Checks sequential with -p 4 (or -p 2 under load).

## 1. TestEveryPublicActionHasAnIdempotencyRow (cmd/metasystem/intent_idempotency_test.go:71)
"landing batch" and "landing restart" have no idempotency row: register each in an intent_idempotency_*_test.go file as read, creation (with why a second call is not a repeat) or stateful (with a witness that runs it twice). Honest classes: `landing batch` (no action word) is a read; `landing batch seal` / `reopen` / `restart` are stateful — the witness runs each twice and asserts the second call answers as applied and writes nothing (R-129, which the unit already implements).

## 2. TestIntentPublicCoverage/public_table (intent_coverage_test.go:85, :114)
"landing restart: summary …, 1 usage lines, 0 examples; all are required" and "public pairs differ": every public action needs a summary, usage lines and at least one example in the coverage table; add them for `landing batch`, `landing batch seal`, `landing batch reopen --with G`, `landing restart`, and reconcile the public pairs the audit lists.

## Checks
Repository audits that every unit MUST run by name before reporting (they are what the full package catches): `go test -p 4 -count=1 -timeout 30m ./cmd/metasystem -run 'TestEveryPublicActionHasAnIdempotencyRow|TestIntentPublicCoverage|TestIntentDeclaredCheck|TestVerbRatchet|TestProofAttemptBinaryLaunchesUseSharedIsolation|TestAudit'`. Then `go test -p 4 -count=1 -timeout 30m ./internal/landing/plain ./internal/layering` and `./cmd/metasystem -run 'TestLanding|TestHelp|TestBatch'`; `go run ./cmd/devgate static`. Report: diff --stat, production line count, every exit.
