# lane-recovers-and-pauses U1 correction 1: the pause survives resolutions, lane-file trouble and re-registration; no test reads the live ledger

Working Mode: Implement. Correction after the Opus read (RETURN, 4 material). Worktree uncommitted; change only what this brief names; leave fixes uncommitted; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json.

## F-1 (high): the clock restarts while the goal is still in the lane
`internal/goal/lane_budget.go` ~118-124: any non-empty outcome for the goal's commit ends the stay, but the lane writes outcome "waiting" for the same commit after a resolved or held conflict (internal/landing/plain/resolve.go ~507). End a stay only on `returned`, a landed member or a push (mirror `entriesOf`'s notion of still-waiting). Test: hand-in 08:00, a "waiting" line 09:00, now 12:00 → paused 240 min and the stay still open (the reader's overlay case).

## F-2 (medium-high): any damaged lane file makes every goal's budget on the host unknown
`internal/dispatch/budget.go` ~892-908 and lane_budget.go ~52-71: a torn line in the lane's queue.jsonl or an incomplete lane registration record turns into BUDGET_UNKNOWN for every goal (admission, governed work and the breach custodian refuse). Skip undecodable lines the way `plain.readLines` does; an unreadable lane registration means "no lane pause" (today's behaviour) with a problem line; fail only when the bad data concerns this goal's own stay. Tests: the two reproductions (torn last line; CustodyEpoch 0) → budgets of goals never in the lane stay known.

## F-3 (medium): closed pauses vanish when the lane registration changes
Nothing is stored; both values are recomputed from the currently registered lane's files; `landing unset` deletes the record. Store the CLOSED paused minutes on the goal's budget record (or ledger) when a stay ends (`lane_paused_minutes`), and compute only the open stay from the lane (`lane_paused_at`); the brief's wording. Test: a closed stay survives `landing unset` and a re-registration at another checkout.

## F-4 (medium): a test reads the live ledger and testing.json
`cmd/metasystem/lane_budget_view_test.go` TestLaneBudgetReadsRealLedgerQueueAndContractPlan reads ../../plans/goals/lane-first-run-fixes.md (goes away when the goal concludes) and the live testing.json. Copy the goal record into testdata/ like the other fixtures; drop the test-plan selection part.

## Checks
`go vet ./internal/goal ./internal/dispatch ./cmd/metasystem`; `go test -count=1 -timeout 30m ./internal/goal ./internal/dispatch ./internal/landing/plain ./internal/config ./internal/layering`; `go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestGoalBudget|TestBreach|TestGoalShow|TestLaneBudget|TestLanding|TestSkill|TestAudit'`; `go run ./cmd/devgate static`. Report: diff --stat, production line count (≤250), every exit, each mutation.
