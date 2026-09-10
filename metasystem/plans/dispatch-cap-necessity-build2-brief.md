Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal dispatch-cap-necessity)
Date: 2026-09-10

# Goal

Goal dispatch-cap-necessity (Wido's highest priority, R-49-m1b): land
the reservation settlement box that chain dispatch-cap-build1-20260909
built and its two critics closed (dispatch-cap-crit2-20260909: no
material finding). That chain cannot land: main gained 1b12f534 ("Make
testing risk-selected and retain proof for landing"), which touches
internal/dispatch/budget.go, after the chain's base, its certified diff
no longer applies, and the chain was closed before that was noticed.
This chain starts from today's main and carries the same box forward.

# What to do

1. The built box lives in the closed chain's own worktree, which the
   dispatcher keeps beside yours under the agents artifacts directory,
   named after that chain (dispatch-cap-build1-20260909). Its working
   tree carries the box as uncommitted changes on the old base: nine
   modified files under internal/dispatch and internal/obligationstate
   plus one new file, admission_test.go beside admission.go. Obtain the
   box as a patch from there (`git diff HEAD` in that worktree, plus
   the new file), and apply it onto your worktree with a three-way
   merge. If your sandbox cannot read that sibling worktree, stop and
   say so in your return; do not rebuild the box from the design.
2. Resolve every conflict so that the box is preserved exactly on top
   of what 1b12f534 changed: take main's side for everything that is
   not the box, the box's side for everything that is, both where they
   meet, and list every conflict you resolved in your return.
3. No new behaviour, no new file beyond admission_test.go, no test
   weakened. The box is the ACCEPTED box of
   metasystem/plans/dispatch-cap-settlement-design.md revision 4 within
   metasystem/plans/dispatch-cap-settlement-scope-cut.md, as the landed
   brief metasystem/plans/dispatch-cap-settlement-build-brief.md binds
   it, with the seat's amendments from the closed chain: the refusal
   line renders the breach fields joined with ", ", then "; " and the
   reserved segment, then "; " and the setup-refusal rule clause where
   it attaches today; T10 asserts those whole lines; the sum-invariant
   test's governed case asserts one active job.

# Proof

`go build ./...`, `go test ./internal/dispatch/ -count=1` (coverage at
or above 75.9 percent), `./internal/obligationstate/`, gofmt, vet, with
a task-specific GOCACHE if the sandbox denies the default. Report the
round as your own.

# Constraints

Wall-clock budget: 30 minutes. Version-2 implementer return with the
complete diffBoundary. Stop at a gap that needs a decision no page has
made; report it with the resolution you propose. Never delete written
work.
