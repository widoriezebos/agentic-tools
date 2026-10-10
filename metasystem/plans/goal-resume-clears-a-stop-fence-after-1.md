# Hot-fix correction 1: a person's resume of a fenced claim passes the one-claim check, and the live "nothing to resume" is reproduced and fixed

Working Mode: Implement. Correction of the hot-fix unit `plans/goal-resume-clears-a-stop-fence.md` after its Opus read. The worktree holds the unit uncommitted on origin/main 0e0feb85f; change what this brief names.

## F-1 (breaking): the one-claim check refuses a person's resume of a fenced claim

`internal/goal/stop.go` ~490-492 calls `checkFenceLiftForRebudget`, which at ~418-432 refuses when the same machine holds any other claimed goal without a fence that is not waiting to land ("machine m1e already holds live claim lane-proves-at-the-batch-risk; conclude, park or release ... first"). Live: m1e holds lane-proves-at-the-batch-risk (claimed, unfenced) next to the two fenced goals, so the person stays in the loop. Fix: a PERSON's `goal resume` of a fenced claim clears the fence without the one-claim check (the check protects agents from running two working claims; a person resuming to conclude or re-hand-in a landed/returned goal is not that case); record the resume with the fence id and reason. Test: a bed where the machine holds another unfenced claim; the person's resume of the fenced goal succeeds and `goal done --reason` then succeeds (mutation: keep the check -> refused, test fails).

## F-2 (material): the claimed-path change was a no-op and the live defect is not reproduced

At HEAD a fenced claim (`IsFencedClaim`, file.go:106: State claimed + Claimed + StopFence) already routes to the resume path that clears the fence through bindClaim; the live goals nevertheless answered "G is running under its standing box; there is nothing to resume" (intent_goals.go ~915) on 2026-10-10 18:1x from the m1e checkout at f60331ae7+ledger. Reproduce it for real: clone origin/main at 0e0feb85f into a throwaway directory (the two live goal files are in it: plans/goals/fleet-provider-and-session-recovery.md and plans/goals/process-changes-cover-declarations-and-interventions.md, both `State: claimed`, `Claimed: machine=m1e ...`, `StopCapability: generation=5 revision=5 machine=m1e claimEpoch=9 fenceEpoch=1`, `StopFence: stopId=stop-<goal>-r5-f1 revision=5 epoch=1 capabilityGeneration=5 ... reason=ELAPSED_LIMIT`), build the engine there and run `bin/metasystem goal resume fleet-provider-and-session-recovery` (no local conf needed for a read; if a verb needs the enrolled terminal, report the exact refusal). Find which projection or routing answers "nothing to resume" for these files (hypotheses: the ledger projection used by intent_goals.go ~915 does not carry StopFence; the claim epoch 9 vs fence epoch 1 binding makes the fence invisible; a machine/lineage check) and fix the cause so that a person's resume clears these two goals' fences. Test: a fixture built from these two headers (copy the header lines) resumes and clears the fence; the test must FAIL at HEAD 0e0feb85f (verify by running it against a clean export).

## Keep from round 1 what still holds; drop what is a no-op

Keep the history record; keep or drop the abandoned-goal path as the fix requires (the read flagged the file.go:843-845 validation deletion as a weakening: restore it unless the abandoned path needs it, and say which).

## Checks

`go test -count=1 -timeout 30m ./internal/goal`; `./cmd/metasystem -run 'TestGoalResume|TestResume|TestBreach|TestStopFence|TestGoalDone|TestClaimBinding|TestGoalStop|TestIntentResume|TestGoalsync|TestCustodian'`; `go run ./cmd/devgate static`; `./internal/audit`. Never edit testing.json; never open metasystem.conf.local; do not commit. Report: the reproduction's exact output before/after, diff --stat, each exit.
