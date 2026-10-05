# Brief: health-is-green-when-the-seat-is-healthy, unit handin-card-quiet (a hand-in without a live card says nothing about the card)

Working Mode: Implement
Goal state: claimed under the seat lineage on m1l, tier 2, box 2d/12/1200m/2/20; coordinated by m1e in Wido's word (2026-10-04 22:25). One committed read (Claude Opus 5.5); no per-round read.

# Goal

Unit board-follows-the-lane (commit 70f3a866 on this branch) makes the lane hand-in write the goal's board card as "joined" and returns the card write's outcome in the result's `Details` (`cmd/metasystem/landing_plain.go`, `inv.writeJoinedCard(goalID)`). When the seat has no single live card for the goal (a records hand-in after the claim was released; the layout fixture of `work land --records`), the hand-in now prints the line "the hand-in card for G was not written: no single live card holds the goal". The committed read of that unit noted this as noise (N-2-HANDIN-NO-CARD-NOISE), and the lane proved the branch red on main's layout golden `TestAuditOutputLayoutJSONUnchanged/work-land-records` (`cmd/metasystem/audit_output_layout_test.go:360`): the JSON result gained a `details` array with that line.

## Decisions on the lane's return

| Finding | Decision | Where |
| --- | --- | --- |
| lane red: TestAuditOutputLayoutJSONUnchanged/work-land-records — the hand-in result gained a details line when no card exists | accepted: a hand-in with no live card writes nothing and says nothing about the card; the layout golden stays as on main | this unit |

# Workspace

The goal worktree of health-is-green-when-the-seat-is-healthy at its tip (units board-follows-the-lane and stuck-detector committed with their reads). Leave the change uncommitted.

# Units

| Unit | Lines |
| --- | ---: |
| handin-card-quiet | 60 |

## What this unit does

1. `writeJoinedCard` (or its caller in the hand-in) distinguishes three cases: a single live card exists and was written (no detail line, as today); a single live card exists and the write failed (one detail line naming the failure, as today); no single live card holds the goal (nothing written, NO detail line: this is the normal state of a records hand-in or a released goal, not a defect to report). The same for the landed-card write on the push side if it prints the same sentence.
2. Tests: the hand-in with no live card returns no `Details` (mutation: print the line); the hand-in with a live card whose write fails still names the failure (mutation: drop the line); `TestAuditOutputLayoutJSONUnchanged/work-land-records` passes unchanged (do not edit the golden).
3. Run the check below; report the exits, `git diff --stat`, and the test list with the mutation each catches.

## Not in this unit

Any change to the card's content or the stuck detector; any golden file; any file outside `cmd/metasystem/landing_plain.go`, the card writer it calls and their tests; any rebase; no receipts, memory or records files. Do not run the whole `cmd/metasystem` package.

# Constraints

Go only; no new dependencies; tests use the plain-lane and board beds, no real Git, model or process environment; every new test calls `t.Parallel()`; `-timeout 30m` on every go test line.

Maximum reader tool calls: 25

# Expected Return

The exits, the diff stat, and the test list with mutations.

# Acceptance Criteria

- `go test -count=1 -timeout 30m -run 'TestAuditOutputLayout|TestWorkLand|TestRecords|TestLandingPlain|TestHandIn|TestAudit|TestEvery|TestVerbRatchet|TestRepository|TestFrozenPublic' ./cmd/metasystem/` passes.
- `go test -count=1 -timeout 30m ./internal/board/ ./internal/landing/plain/` passes.
- `go run ./cmd/devgate static` exits 0.
- Tests: no detail line without a live card (mutation: print it); the failure line stays when a live card's write fails (mutation: drop it).
