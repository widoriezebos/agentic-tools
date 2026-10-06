# Brief: lane-lands-finished-goals, unit next-step

Working Mode: Implement
Goal: lane-lands-finished-goals. Accepted design: plans/designs/lane-lands-finished-goals.md, Decision 1, section "The next step after a unit". Builds on unit admission (already committed on this branch): use its `goalProgress` function; do not duplicate its logic.

# Goal

After a unit's read is published, or after a tier-1 unit is committed, the next step a person or agent sees is the next unit of the goal, or the goal's one hand-in when it is finished; never `work land` for a goal that is not finished.

# Workspace

The goal worktree /Users/wido/LocalStorage/GitHub/agentic-tools-m1e-lane-lands-finished-goals on branch goal/lane-lands-finished-goals, with unit admission committed. Leave this change uncommitted.

# Units

| Unit | Lines |
| --- | ---: |
| next-step | 70 |

## What this unit builds

1. One function, `goalNextStep(goalID)` (beside `goalProgress`), returning the next argv and its reason from `goalProgress`'s answer:
   - a declared unit not built: `work build G --work UNIT --brief FILE --check COMMAND` (the form `work build` documents, `cmd/metasystem/intent_work.go:226`), reason naming the unit;
   - a unit built that lacks its read: `work review G --work UNIT`;
   - no end declared: `work build G --work NAME --brief FILE --check COMMAND`, reason "goal G has no Units table; this builds its next unit, and `--last` marks its last one";
   - finished: `work land G`, reason "goal G is finished: every unit it declares is built and read. This is its one hand-in; `goal done` follows when it has landed."
2. The seven sites that print `work land` as the next step after a unit use it: `cmd/metasystem/intent_selection.go:281` (tier-1), `:285`, `:358`, `:627`; `cmd/metasystem/intent_unit_review.go:399`; `cmd/metasystem/intent_manual_submit.go:339` (tier-1), `:363`. The hint at `cmd/metasystem/intent_delivery.go:1894` stays (it follows a successful hand-in). For a goal whose reads are waived, after a unit that is not the last the tier-1 sites print the next unit's build.
3. Every new reason text passes the message-trace audit.

## Not in this unit

`--last`, the `Goal-Whole` trailer, `work land --whole` (unit boundary); the `landing_plain.go:67` prose (later units); anything in `internal/`; plan, docs, receipts, memory or records files.

# Readers of what this unit changes

The seven sites and their callers (`manualContinuation`, `workContinuation` in `intent_selection.go`; `runIntentStatusGoal` uses `workContinuation` at the status lines), `intent_unit_review.go` review result, `intent_manual_submit.go`; tests asserting `work land` as the next step (grep `"work", "land"` and `work land` in `cmd/metasystem/*_test.go`), which change to the next unit or stay for a finished goal; `TestAuditMessagesTraced` and the layout goldens.

# A test through the public verb

`work review G --work U2` on a goal whose design declares U1, U2, U3 with U3 unbuilt prints `work build G --work U3 ...` as next (mutation: print `work land`, red); with U3 built and read, `work review G --work U3` prints `work land G` (mutation: print the build, red); a tier-1 goal after its first of two units prints the second unit's build.

# Constraints

Go only; no new dependencies; every new test calls `t.Parallel()`; `-timeout 30m` on every go test line; a test changed because the rule changed says so in the return.

Maximum reader tool calls: 20

# Expected Return

The exits, `git diff --stat`, the tests added and changed with the mutation each catches.

# Acceptance Criteria

- `go test -count=1 -timeout 30m -run 'TestWorkLand|TestWorkReview|TestIntentWorkReview|TestUnitReview|TestManual|TestIntentStatus|TestSelection|TestGoalProgress|TestGoalNextStep|TestAudit|TestEvery|TestVerbRatchet|TestRepository|TestFrozenPublic|TestHCL' ./cmd/metasystem/` passes; `go run ./cmd/devgate static` exits 0.
- No site prints `work land` for a goal that is not finished.
