# Unit U2 correction 3 (ruling defect, by the steward): a lane fix never runs on a goal a person parked or un-approved

Working Mode: Implement. Correction of U2 of lane-recovers-and-pauses after its machinery read (code-critic-0f34260e9082fba591745f56, finding RULING-R-142-m1e, material). Worktree at commit b04113a0b; make the change uncommitted on top; never commit, rebase or push; never touch memory/receipts.log or plans/.

## The defect
`cmd/metasystem/intent_work.go` ~932-1001: the `laneFix` switch skips not only the goal's BUDGET checks (review-round limit, elapsed, attempts — what design D2 allows) but also the checks on the goal's STATE (`file.State != goal.StateClaimed`, ~938 and ~947) and on whether its approval is still current (~966/1001). So a lane fix build or read runs on a goal a person has parked or whose approval was withdrawn while it sat in a red batch — a machine rule overriding a person's decision (R-142-m1e, "NO HAL9000"). Fix: the lane allowance replaces ONLY the budget-box admission; a lane fix still requires the goal claimed (not parked, not withdrawn, not done) with a current approval, exactly as any build. Apply the same rule to the lane fix's read admission (goal_branch.go ~181-196 goal-free critic path: it must also refuse when the goal is not claimed with a current approval).

## Test
A goal in a running red batch is parked by a person (`goal park` → StateParked, live); the lane fix build is refused with the ordinary state refusal and no job starts; the lane fix read is refused the same way; un-park (claim again) → admitted. Mutation: drop the state check → the test fails. Keep `TestLaneFixLapsedGoalBoxAdmitsBuild` and `…CommittedReadRealDispatch` green (a lapsed BUDGET still admits).

## Checks
`go vet ./cmd/metasystem`; `go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestLaneFix|TestSkill|TestAudit'`; `./internal/layering`; `go run ./cmd/devgate static`. Report: diff --stat, the unit's production line count (≤250), every exit, the mutation.
