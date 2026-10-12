# lane-batching U4 correction 1: repeats are success, the headline never hides a question, the seal is the person's or U1's, no live-ledger test

Working Mode: Implement. Correction after the Opus read (RETURN, 4 material). Worktree uncommitted; change only what this brief names; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json. Checks sequential with -p 4 (or -p 2 under load).

## F-1 (R-129-ui): repeated verbs write again
internal/landing/plain/batch_verbs.go ~38-41, ~61-81: `seal` on a sealed batch overwrites SealedAt and appends a record; `reopen --with G` while G is already a member re-stamps and even pushes a running batch back to prepared when no proof process is alive; a repeated `restart` appends a second pending action. Fix: an act whose effect already holds answers "already sealed" / "already a member" / "request already pending" as applied and writes nothing (TestLandingBatchVerbsRecordPersonAndClock's sealed/seal and running/seal cases expect the ORIGINAL SealedAt and no second record).

## F-2 (high): the headline hides a person's open question while a proof runs
internal/landing/plain/status.go ~533-535 sets status.Summary = ProofHeadline unconditionally, overriding the open-question headline (~508), the hand-in problem (~518) and the depth-class/overdue suffix (~525-531). Fix: delete those lines; ~527 already makes the proving headline the summary unless a question or problem outranks it (the "never idle" requirement holds). batch_status_test.go ~41-50 asserts the opposite — flip it: the question headline wins while proving. Fold: `Lane: idle; its landing agent starts when there is work` (cmd/metasystem/intent_landing.go ~563) and `agent: idle since` (agent_step.go ~150) also print while a proof runs — the Lane line says "Lane: proving; the agent waits for the keeper" in that state; the agent line stays factual (it IS idle) — only the Lane line changes.

## F-3: SealedAt stamped at gate admission collides with U1's open window
internal/landing/plain/batch.go ~761-763 writes SealedAt at gate admission; the amended D1 says the seal is stamped when the FULL proof starts (U1 owns that stamp). Fix: delete those lines; SealedAt is set only by the person's `seal` here (BatchLines already falls back to running.Since).

## F-4: a test reads the live goal ledger
cmd/metasystem/landing_batch_verbs_test.go TestLandingBatchRealContractPlan globs ../../plans/goals/*.md and fails the package on any goal's parse problem — nothing to do with the verbs. Delete it.

## Folds (non-material)
- The pending reopen/restart record carries the running attempt id (`"attempt"`) so U2 can ignore a request whose attempt ended; status text lists pending requests under the batch ("restart requested by Wido at HH:MM for attempt X; nothing acts on it until U2 lands").
- The pending message says plainly "recorded; nothing acts on it until the proof ends — run reopen again then".
- The not-waiting refusal names the goal's state (returned / landed / never handed in).
- Help grouping: `batch seal` and `reopen` under the writing verbs.

## Checks
`go vet ./cmd/metasystem ./internal/landing/plain`; `go test -p 4 -count=1 -timeout 30m ./internal/landing/plain ./internal/layering`; `go test -p 4 -count=1 -timeout 30m ./cmd/metasystem -run 'TestLanding|TestAudit|TestHelp|TestBatch'`; `go run ./cmd/devgate static`. Report: diff --stat, production line count (≤250 total), every exit, each mutation.
