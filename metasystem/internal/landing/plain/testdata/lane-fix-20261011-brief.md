# Lane fix round 1: process-changes-cover-declarations-and-interventions

You are the seat of goal `process-changes-cover-declarations-and-interventions` (tip
`625ec7bc7953f00a221f8965fccb67dcc702438b`), working in the landing lane checkout. Its detached
HEAD is the batch commit `d2ede1007a44fb92d9b21e4d8c4c971c035b8f3a`, which merges the goal on
main `7481259dd338f241bfe50480370329d988e9ffb5`. The lane's full proof of that tree is red, and
the proof blames the goal. Main's own full proof (`3c01896`, trunk) was green, so these reds come
from the goal's change on today's main.

Rule: fix the goal's code or its tests; never loosen or delete a test; if the fix needs a decision of the goal's person, stop and say so.

## Failed units and tests

Proof attempt `20261011T040737.033326000Z`, full scope, `sh proof/full.sh`.
Proof log: `artifacts/agents/landing/proofs/20261011T040737.033326000Z.log`

Unit `metasystem/cmd/metasystem`:

1. `TestEveryPublicActionHasAnIdempotencyRow` (`cmd/metasystem/intent_idempotency_test.go`)
   > intent_idempotency_test.go:71: "work revert" has no idempotency row: register it as read,
   > creation (with why a second call is not a repeat) or stateful (with a witness that runs it
   > twice) in an intent_idempotency_*_test.go file

   The goal added the public action `work revert` (`cmd/metasystem/intent_work_commit.go`,
   `intent.go` Work group). Register its row honestly for what the verb does (see
   `cmd/metasystem/intent_idempotency_work_test.go`); if it is stateful, its witness must run it
   twice and show the repeat succeeds.

2. `TestEveryStatefulActionRepeatsAsSuccess` and `TestEveryStatefulActionRepeatsAsSuccess/work_build`
   (`cmd/metasystem/intent_idempotency_test.go`, row in `intent_idempotency_work_test.go`)
   > intent_idempotency_work_test.go:121: work build unit records: the repeat changed files:
   > [.../002/unit/.named/f8e87d0ca1770406631988adde387b4a.plan]

   A repeated `work build` now rewrites a named plan file. The goal changed the build path
   (`intent_unit_check.go`, `intent_work.go`, `intent_worktree.go`, plan publication). A repeat
   must not change files.

3. `TestFleetBuildGrantProbeDoesNotRefuse` (`cmd/metasystem/fleet_build_admission_test.go`)
   > fleet_build_admission_test.go:538: admitted build logged a refusal:
   > 2026-09-01T10:00:00Z refused grant=fixture-grant by=Wido act="committed unit checks"
   > reason="set only by the person's own proof"

   An admitted fleet build now logs a refusal for "committed unit checks". Look at how the goal's
   unit-check selection (`intent_unit_check.go`) probes or consumes the grant for committed checks.

4. `TestWorkRebaseGitAdapterHoldsAfterHistory` (`cmd/metasystem/intent_delivery_owner_test.go`)
   > intent_delivery_owner_test.go:549: work land: code=1 ... "outcome": "refused",
   > "summary": "goal standing-validation is not finished: its design declares 1 units; u1 has no
   > clean read. A goal lands whole, once." ... next: metasystem work review standing-validation --work u1

   `work land` in this fixture now finds unit u1 without a clean read. The goal changed
   `intent_delivery.go`, `intent_work_rebase.go`, `intent_work_commit.go` and
   `internal/goal/branch/commit.go`; find which change stops the fixture's read from counting.

Read each failing test's full output in the log (`grep -F '"Test":"<name>"' <log> | jq -r
'select(.Action=="output") | .Output'`).

## Change under test

`git diff b676421675295645faa108589e4bdbb41b129f06..HEAD -- metasystem` (batch base main to the
batch commit; it also includes main's own 14 commits since that base). The goal's change on the
current main alone is `git diff HEAD^1..HEAD -- metasystem`. The merge resolution of
`intent_delivery.go`, `intent_unit_check.go` and `internal/goal/branch/commit.go` was job
`lane-merge-e54f64cc3a2fc0a7e06cb780568151e398529a9661e3d1ba218a` (read
`read-9d0df69d0d6e6fcc799d99b7`); main's side in those files must stay whole.

## Check

From `metasystem/`: `go build ./...`, `go vet ./cmd/metasystem`, then in `./cmd/metasystem`
`go test -run 'TestEveryPublicActionHasAnIdempotencyRow|TestEveryStatefulActionRepeatsAsSuccess|TestFleetBuildGrantProbeDoesNotRefuse|TestWorkRebaseGitAdapterHoldsAfterHistory' ./cmd/metasystem`
and the goal's own tests (`-run 'TestIntent(Carry|Declaration|Inverse|NestedCheck|ProcessCheck|ProcessReport)'`).
Then the unit check the engine names. Leave `records/narrator-digest.log` and the untracked
`process/` directory alone; they are the lane's, not part of this fix.

## Units

| Unit | Paths | Estimated changed lines |
|---|---|---|
| lane-fix-1 | `metasystem/cmd/metasystem/intent_idempotency_work_test.go`, `metasystem/cmd/metasystem/intent_work_commit.go`, `metasystem/cmd/metasystem/intent_unit_check.go`, `metasystem/cmd/metasystem/intent_work.go`, `metasystem/cmd/metasystem/intent_delivery.go` | ~120 |
