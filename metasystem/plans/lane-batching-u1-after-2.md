# lane-batching U1 correction 2: records-only members stay, no newcomer during a fix round, no live goal pages in tests

Working Mode: Implement. Correction after the Opus read (RETURN, 3 material); the unit's last correction. Worktree uncommitted; change only what this brief names; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json. Checks sequential with -p 4 (or -p 2 under load). The design copy in plans/designs/lane-batching.md is amended (D1): read it first.

## F-1 (high): records-only hand-ins can never land under auto
internal/landing/plain/batch.go ~452-454 drops records entries from the selection, ~506 skips them as newcomers, status.go ~533 hides them. The base selected them as members (a records hand-in "is proved and pushed by the landing agent"); D7 only says they never START or RESTART a proof. Fix: remove both filters and the status exclusion (base behaviour restored); a records-only member rides the batch. Test: a single records hand-in under auto → a prepared batch with that member (mutation: restore the filter → nil batch → test fails).

## F-2 (high): a newcomer during a red-gate fix round wedges the batch
A fix round's running record snapshots BatchMembers at admission; a newcomer added while the fix round is open makes checkBatchLocked (batch.go ~708) and advanceFix (fix.go ~231) refuse the fix commit ("does not merge a selected member"). Fix (smallest): no newcomer joins while a fix round is open on the batch (the window pauses; status says "G waits: a fix round is open"); it joins when the round ends if the batch is still open. Test: open batch, red gate, fix round open, hand-in → not a member, reason printed; round ends → joins (mutation: admit during the round → the fix commit is refused → test fails).

## F-3 (medium): a test pins two live goal pages
cmd/metasystem/landing_check_skip_test.go ~231 requires plans/goals/lane-first-run-fixes.md and lane-rehearsal-before-landing.md to exist with a tier; `goal done` deletes a goal page, so main would go red on an unrelated act. Fix: take ids from the tiers map already read from the live pages, or synthetic ids through the GoalTier seam; never a named live goal.

## Folds (non-material, cheap)
- `landing status` must stay a read: it runs a selection (with a `git fetch` of main) on every read and can close or prepare batches. Move the selection to where the lane acts (`landing run` / the keeper tick / the agent's step); status only reports. If that is not a small change, say so and leave it for U2's keeper work.
- A reused saved green never stamps the seal: stamp sealed_at when the full proof's result is reused too.
- The newcomer refusal's next step names the gate ("landing prove --gate --wait, then landing prove --wait").
- Remove the stale comment "One goal per batch until…" in metasystem.conf (no key under it) — only if metasystem.conf edits are allowed for a unit; otherwise report it.

## Checks
`go vet ./internal/landing/plain ./cmd/metasystem`; `go test -p 4 -count=1 -timeout 30m ./internal/landing/plain ./internal/layering`; `go test -p 4 -count=1 -timeout 30m ./cmd/metasystem -run 'TestLanding|TestSkill|TestAudit|TestBatch'`; `go run ./cmd/devgate static`. Report: diff --stat, production line count (≤250 total), every exit, each mutation.
