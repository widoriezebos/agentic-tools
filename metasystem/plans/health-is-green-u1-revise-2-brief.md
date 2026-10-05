# Brief: health-is-green-when-the-seat-is-healthy, unit board-follows-the-lane, round 3: re-run the proof; the environment moved a ref during round 2

Working Mode: Implement
Goal state: claimed under the seat lineage on m1l, tier 2, box 1d/10/1200m/1/20; coordinated by m1e in Wido's word (2026-10-04 22:25). One committed re-read (Claude Opus 5.5) follows this round; the round's own read is not run.

# Goal

Round 2 removed the one wall-clock call the lane found (`internal/board/card_test.go:52`, `time.Now().Add`) and its check PASSED, but the runner recorded the round as `proof-wrote`: between its before and after snapshots one Git ref outside the change moved (`refs/heads/goal/agent-works-as-project-partner`, fetched by the installation's steward re-arm while the check ran). Nothing is wrong with the change. This round re-runs the check so the round ends clean.

## Decisions on round 2

| Finding | Decision | Where |
| --- | --- | --- |
| proof-wrote: refs/heads/goal/agent-works-as-project-partner moved during the check | refuted (environment: a fetched ref of another goal, not the unit's) | no change |
| lane return: card_test.go:52 wall-clock call | fixed in round 2 | internal/board/card_test.go |

# Workspace

The goal worktree of health-is-green-when-the-seat-is-healthy, branch goal/health-is-green-when-the-seat-is-healthy, with round 2's change in it (uncommitted). Leave it uncommitted.

# Units

| Unit | Lines |
| --- | ---: |
| board-follows-the-lane | 60 |

## What this round does

1. Run the unit's check exactly as the plan states it (the `proof` entry of the run's plan; the same command round 2 ran), from `metasystem/`; add `go test -count=1 -timeout 30m -run 'TestNoTestWaitsOnWallTime' ./internal/testenv/` once, since that is the test the lane failed on.
2. Green: stop; report the exits and `git diff --stat`. Do not touch a file.
3. Red: fix only what the red names inside the files round 2 changed, re-run, report.

## Not in this round

Any new behavior; any file round 2 did not change; any rebase; no receipts, memory or records files.

# Constraints

Go only. No real Git in tests, no process environment set by tests, no wall-clock calls in tests. `-timeout 30m` on every go test line.

Maximum reader tool calls: 30

# Expected Return

The exits, the diff stat, and either "unchanged" or the fix with its red.

# Acceptance Criteria

- The plan's check and the wall-time test are green on the worktree as it stands; no file outside round 2's change is modified.
