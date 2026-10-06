# Brief: lane-lands-finished-goals, unit admission

Working Mode: Implement
Goal: lane-lands-finished-goals (tier 3, claimed on m1e, box 4d/40/3000m/2/30). Accepted design: plans/designs/lane-lands-finished-goals.md, Decision 1 ("a goal is handed in whole, and once"). This unit builds the admission half of Decision 1; the next-step function (unit next-step) and the `--last` / `Goal-Whole` / `--whole` boundary (unit boundary) are later units.

# Goal

`work land G` hands in a goal only when it is finished, and only once.

# Workspace

The goal's worktree from `work build`, at main. Leave the change uncommitted.

# Units

| Unit | Lines |
| --- | ---: |
| admission | 110 |

## What this unit builds

1. **`goalProgress`**, one function in `cmd/metasystem` (beside `handLandingSubject`, `cmd/metasystem/intent_delivery.go:2217`), reading the goal's declaration and its branch: the declaration is the Units table of the goal's first accepted design page that has one, found exactly as build admission finds it (`acceptedDesignPaths`, `cmd/metasystem/intent_work.go:1024`; `launch.DeclaredUnits`, `internal/launch/admit.go:457`; a unit name matches a row by the rule of `sizesFromTable`); the branch is `state.Status.Units`, `state.Status.Prefix` and `state.ReadsWaived` (`intentBranchState`, `cmd/metasystem/intent_delivery.go:296-306`). A unit is finished when it has a commit on the branch and either a clean read or the goal's reads are waived (`goal.ReadsWaived`, `internal/goal/landgate.go:663`). It answers one of four results: no end declared (no Units table; unit boundary adds `--last` later); the first declared unit in table order that is not finished, and whether it lacks its build or its read; a unit on the branch outside the declaration that lacks its read; finished.
2. **The rule in `landGoalRoute`** (`cmd/metasystem/intent_delivery.go:1899`), placed beside the `handLandingSubject` call (`cmd/metasystem/intent_delivery.go:2030`) before the route splits, so the lane route and the hand route share it. `work land G` (no `--records`, which is untouched) refuses when `goalProgress` is not finished; when `--through COMMIT` names anything but the branch's last unit commit (a goal lands whole); when the goal's newest hand-in already landed and the branch has unit commits beyond it (a goal lands once; `plain.Latest` and `plain.Landed`, `internal/landing/plain/queue.go:223`, `:346`; on the hand route the landed tip as `landByHand` records it). A waiting line replaced by a newer tip, and a returned goal handing in again, stay allowed. A branch that holds only plan commits of a goal without a table keeps today's behaviour (`cmd/metasystem/intent_delivery.go:2225`).
3. **Refusal text**, two lines each, as the design's "What a person reads" block gives them (the not-finished line naming the unbuilt and unread units and `work build ... --work UNIT --brief FILE --check COMMAND` as the next step; the no-end line with the build of the next unit; the landed-once line with `goal done G --reason TEXT`). Every new refusal has a refusal code registered in `internal/refusal/register.go` and passes the message trace audit.
4. `handLandingSubject`'s `landable` arithmetic (`:2238-2246`) is replaced by `goalProgress` for a goal hand-in; its other callers, if any, keep working (grep and list them).

## Not in this unit

The eight next-step sites (unit next-step); `--last`, the `Goal-Whole` trailer, `work land --whole --by` (unit boundary); the rebase (unit no-rebase); anything in `internal/landing/`; plan, docs, receipts, memory or records files.

# Readers of what this unit changes

`handLandingSubject` and its callers (grep `handLandingSubject` in cmd/metasystem); `landGoalRoute`; `acceptedDesignPaths` and `launch.DeclaredUnits` (read-only use); `intentBranchState`; the tests that hand in part of a goal today and must change to finished goals or to the new refusal: `cmd/metasystem/landing_plain_handin_test.go`, `cmd/metasystem/landing_plain_cards_test.go`, `cmd/metasystem/landed_notice_test.go`, `cmd/metasystem/intent_records_test.go`, `cmd/metasystem/landing_rebase_test.go`, `cmd/metasystem/intent_delivery_owner_test.go` (its "a hand-in never moves main" assertion must keep passing unchanged); list every test you change with why. A test fixture goal that hands in work gains a design page with a Units table naming its units, or is built so its units are all finished.

# A test through the public verb

`work land` on a goal whose accepted design declares three units and whose branch has two finished: refused, names the third (mutation: skip the `goalProgress` check, red). With all three finished: hands in. A new unit commit after the goal landed: refused with the landed-once line (mutation: drop the landed check, red). `--through` naming the second of three: refused. A tier-1 goal (reads waived) with all declared units built and unread: hands in.

# Constraints

Go only; no new dependencies; every new test calls `t.Parallel()`; `-timeout 30m` on every go test line; never weaken or delete a test's intent; a test changed because the rule changed says so in the return.

Maximum reader tool calls: 20

# Expected Return

The exits of the check, `git diff --stat`, the tests added and changed with the mutation each catches, and the list of `handLandingSubject` callers.

# Acceptance Criteria

- `go test -count=1 -timeout 30m -run 'TestWorkLand|TestLandingPlain|TestHandIn|TestLanded|TestRecords|TestIntentDelivery|TestIntentLand|TestAudit|TestEvery|TestVerbRatchet|TestRepository|TestFrozenPublic|TestHCL' ./cmd/metasystem/` passes; `go run ./cmd/devgate static` exits 0.
- A partial goal cannot be handed in on either route; a finished goal can; a landed goal cannot hand in again.
