# Unit U2 correction 2 (last): no test reads the goal's live ledger page; settings show finds the two keys by name

Working Mode: Implement. Correction 2 of U2 of lane-recovers-and-pauses after its machinery-recorded read (code-critic-5fdcb7566b96fef8f986a393). Worktree at commit b4ac32d29 (U2 committed); make the changes uncommitted on top; never commit, rebase or push; never touch memory/receipts.log or plans/.

## F-1 (material): a time-bombed test
`cmd/metasystem/intent_lane_fix_test.go`: `TestLaneFixLapsedGoalBoxAdmitsBuild` and `TestLaneFixLapsedGoalBoxAdmitsCommittedReadRealDispatch` read this goal's live ledger page `plans/goals/lane-recovers-and-pauses.md` and fail when it is missing; `goal done` moves that page to records/goals/, so the tests go red on main the day the goal concludes. Fix: the budget shape comes from a fixture the test writes itself (or a testdata copy of a goal page with the lapsed box), never from a live ledger page. Mutation: delete the fixture → the test fails with its own message, not a missing-ledger error.

## F-2 (fold, small): `settings show KEY`
`settings show landing.fix-rounds` (and fix-minutes) finds the key like any other launch setting (today the two are appended only when `settings show` has no argument, intent_work.go ~2311). Test both forms.

## F-3 (noted, no change): old `lane-fix-<n>` names are refused by the new regex; no live record uses them.

## Checks
`go vet ./cmd/metasystem`; `go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestLaneFix|TestSettings|TestAudit'`; `go test -count=1 -timeout 30m ./internal/landing/landpath ./internal/layering`; `go run ./cmd/devgate static`. Report: diff --stat, production line count of the whole unit (≤250), every exit, the mutation.
