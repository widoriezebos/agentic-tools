# Brief: lane-lands-finished-goals, unit boundary

Working Mode: Implement
Goal: lane-lands-finished-goals. Accepted design: plans/designs/lane-lands-finished-goals.md, Decision 1, "What declares a goal's end", second bullet. Builds on units admission and next-step (committed on this branch): extend `goalProgress`; do not duplicate it.

# Goal

A goal without a Units table declares its end on the branch: the unit built with `--last` carries a `Goal-Whole: G` trailer; `goalProgress` reads it; a person can declare the end at hand-in with `work land G --whole --by NAME`.

# Workspace

The goal worktree /Users/wido/LocalStorage/GitHub/agentic-tools-m1e-lane-lands-finished-goals on branch goal/lane-lands-finished-goals. Leave this change uncommitted.

# Units

| Unit | Lines |
| --- | ---: |
| boundary | 70 |

## What this unit builds

1. **The trailer.** `CommitRequest` gains a `Whole bool`; `commitMessage` (`internal/goal/branch/commit.go:228`) adds the line `Goal-Whole: G` to a unit commit's trailers when it is set. The branch reader keeps taking a commit's kind from the three trailers it knows (`internal/goal/branch/range.go:141`) and records the mark on the unit: `UnitStatus` (`internal/goal/branch/status.go:10`) gains `Whole bool`, set from the commit's trailers. `work rebase` and the carry keep the trailer (it is in the commit message).
2. **`--last`.** `work build G --work UNIT --last` (`cmd/metasystem/intent_work.go:226` declares `work build`) and the hand-committed route (`cmd/metasystem/intent_manual_submit.go`) pass `Whole` to the unit's commit request. Help text names it: "marks this unit as the goal's last; for a goal whose design has no Units table".
3. **`goalProgress` reads the end.** For a goal without a Units table: the end is the newest unit with `Whole`; units committed after it are part of the goal; the goal is finished when every unit up to and including the marked one, and after it, is built and read clean or waived. Without a marked unit the answer stays "no end declared".
4. **`--whole --by NAME`.** `work land G --whole --by NAME` declares the end at hand-in for a goal whose last unit was built without `--last`: proven as every `--by` act is (`actingAs`, `cmd/metasystem/intent_planning.go:479`, as `incident close` uses it at `:2263`); refused without `--by` or for an agent; the queue line records who declared it (`Line` gains `WholeBy string`, `internal/landing/plain/queue.go:51`); every unit on the branch must still be read clean or waived.
5. The no-end refusal's line 2 (`work build G --work NAME --last ...`) and `goalNextStep`'s no-end reason now name a real flag; add `work land G --whole --by NAME` as the alternative in the refusal's reason text.

## Not in this unit

The rebase (unit no-rebase); anything else in `internal/landing/`; plan, docs, receipts, memory or records files.

# Readers of what this unit changes

`commitMessage` and its callers (grep `commitMessage(` and `CommitRequest{`); `KindOf`/range reading of trailers (`range.go`); `UnitStatus` constructors (`status.go` `InspectStatus`); `work build` flag parsing and `intent_manual_submit.go`; the carry (`internal/goal/branch/rebase.go` carryReviewsWith) keeps messages; `plain.Line` JSON (queue readers tolerate a new optional field); `TestFrozenPublic*` corpora if `work build`/`work land` help is frozen; `TestVerbRatchet` for the new flags.

# A test through the public verb

`work build G --work u2 --last` on a table-less goal writes a commit whose message has `Goal-Whole: G` (mutation: drop the trailer, red); `work land G` on that goal with u1 and u2 read hands in; with u2 unread is refused naming u2; `work land G --whole --by Wido` on a table-less goal whose last unit lacks the mark hands in and the queue line records `wholeBy`; the same without `--by` is refused.

# Constraints

Go only; no new dependencies; every new test calls `t.Parallel()`; `-timeout 30m` on every go test line; never weaken a test; a test changed because the rule changed says so.

Maximum reader tool calls: 20

# Expected Return

The exits, `git diff --stat`, tests added and changed with the mutation each catches.

# Acceptance Criteria

- `go test -count=1 -timeout 30m ./internal/goal/branch/` passes; `go test -count=1 -timeout 30m -run 'TestWorkLand|TestWorkBuild|TestIntentWorkBuild|TestManual|TestGoalProgress|TestGoalNextStep|TestLandingPlain|TestAudit|TestEvery|TestVerbRatchet|TestRepository|TestFrozenPublic|TestHCL' ./cmd/metasystem/` passes; `go run ./cmd/devgate static` exits 0.
- A table-less goal can declare its end on the branch or at hand-in by a person, and cannot land without one.
