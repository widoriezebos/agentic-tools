# lane-rehearsal-before-landing U1 correction 5: the three audits the lane's proof failed

Working Mode: Implement. Correction after the landing lane's proof of this goal merged on main (attempt 20261011T235626, 2026-10-12 02:43) failed three cmd/metasystem audits, all this unit's own. Worktree = this checkout (branch goal/lane-rehearsal-before-landing); leave fixes uncommitted; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json. Checks sequential with -p 4 (or -p 2 under load).

## 1. `landing rehearse` has no idempotency row
cmd/metasystem/intent_idempotency_test.go:71: "landing rehearse" has no idempotency row: register it as read, creation (with why a second call is not a repeat) or stateful (with a witness that runs it twice) in an intent_idempotency_*_test.go file. A rehearsal creates a scratch lane and a fixture goal each time: register it the way the audit asks (read the existing rows for the shape; choose the class honestly and give the witness).

## 2. The rehearsal's test launches a metasystem fixture outside proofBinaryFixture.command
cmd/metasystem/proof_fixture_test.go:155: landing_rehearse_test.go launches a metasystem fixture outside proofBinaryFixture.command. Route the launch through proofBinaryFixture.command (shared isolation) as every other test does.

## 3. Engine self-subprocess sites 111 > ceiling 102
cmd/metasystem/verb_ratchet_test.go:1345 (verbRatchetSelfSubprocessCeiling): this unit added nine engine self-subprocess sites. Prefer removing sites: the rehearsal should launch the candidate engine through ONE helper, not nine call sites; if a site is genuinely needed, raise the ceiling by exactly the sites that remain and say why in the ratchet's comment (the ratchet exists so that every new self-subprocess is a deliberate decision).

## Checks
`go vet ./cmd/metasystem ./internal/landing/...`; `go test -p 4 -count=1 -timeout 30m ./cmd/metasystem -run 'TestEveryPublicActionHasAnIdempotencyRow|TestProofAttemptBinaryLaunchesUseSharedIsolation|TestVerbRatchet|TestLandingRehearse|TestAudit'`; the changed internal packages whole; `go run ./cmd/devgate static`. Report: diff --stat, every exit, and for 3 the site count before and after.
