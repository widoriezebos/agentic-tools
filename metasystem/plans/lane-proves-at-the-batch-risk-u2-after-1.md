# Unit U2 correction 1: the push honours the decided depth; a tierless goal is full; the background child honours the admitted scope

Working Mode: Implement. Correction of unit U2 (brief `plans/lane-proves-at-the-batch-risk-u2-depth-policy.md`, design D1/D2, ruling R-149-m1e) after its Opus read. The worktree holds U2 uncommitted on top of U1 (3a4ad1d1b); change only what this brief names.

## F-1 (material): the push ignores the depth decision

`push.go:143` refuses an impact result only when its base differs from the batch base; `provenGreen` (`push.go:268`) accepts any green. A tier-3 batch decided `full` can push on an impact green: the agent runs `landing prove --impact` (advertised at `intent_landing_prove.go:152`, and `push.go:144` itself prints `Next: metasystem landing prove --impact`). Fix: record the decided depth on the batch's admission/attempt (it is already recorded as the reason: add the decided scope itself), and the push admits an impact green only when the batch's recorded decision for that base is `impact`; a `full` decision with only an impact green refuses with the remedy `metasystem landing prove` (never `--impact`); an explicit `--impact` on a batch whose decision is full records that it does not satisfy the push. Test: tier-3 batch, `prove --impact` green, `landing push` refused naming the decided depth (mutation: drop the decision check -> pushes, test fails); tier-2 batch decided impact, impact green pushes.

## F-2 (material): a goal with no tier widens to impact

`goal.GoalFile.Tier` zero is tolerated during the classification migration (`internal/goal/file.go:31`) and the goal package treats a tierless goal as tier three (`file.go:1734`); `batchDepth` (`test_impact.go` ~294-299) counts tier 0 as lowest. Fix: use the goal package's own effective-tier rule (reuse its function; do not copy): a missing or zero tier is tier 3, so the batch proves full with the reason `tier 3 (unset): full`. Test: a member goal file without a Tier line -> full (mutation: treat 0 as lowest -> impact, test fails).

## F-3 (material, regression): a background proof that fell back to the old scope logic refuses forever when that logic picks `scoped`

`admitExecutionLocked` (`proof_admission.go` ~81) now stores `decision.ScopeReason` on every admission; the background child copies whatever reason it finds into `seams.DepthReason` (`intent_landing_prove.go` ~261); when `batchDepth` returned no decision (no or closed batch, or a member file missing) and `decideScope` chose `scoped`, the child's `proofScope` returns `full` on the non-empty reason (`proof_admission.go` ~52) and `prove.go` ~836 fails with "the admitted check scope changed; request a fresh check", repeating until the full green ages past an hour. Fix: the admission carries the DECIDED SCOPE and a marker that the depth policy decided it (not just a reason text); the child honours the admitted scope exactly (full, impact, scoped, inherited) and never re-derives it from a reason; old-logic reasons are stored as reasons only. Test: no batch decision (member file missing), old logic picks `scoped`, a background `landing prove` (no --wait) admits and runs at `scoped` and ends green (mutation: re-derive from the reason -> "scope changed", test fails).

## Also (not material, one line): a refusal names the real error

`proof_admission.go` ~55-56 overwrites `impactScope`'s error text with the depth reason; keep the error text and append the reason.

## Checks

`go test -count=1 -timeout 30m ./internal/landing/plain ./internal/config`; cmd tests by name (`TestLandingProve|TestLandingPush|TestLandingStatus|TestLandingDepth|TestLandingImpact|TestLandingClock|TestLandingScoped|TestLandingCheck`); every existing test using a seam you changed (grep, by name); `go run ./cmd/devgate static`. Never edit testing.json; never open metasystem.conf.local; do not commit. Report: diff --stat of the correction, each exit, the three scenarios before/after.
