# Unit U4 correction 2 (last): ancestry ignores hand-ins main already contains

Working Mode: Implement. Second and last correction of unit U4 (brief `plans/lane-proves-at-the-batch-risk-u4-depth-class-and-full-due.md`; correction 1 `plans/lane-proves-at-the-batch-risk-u4-after-1.md`). The worktree holds U4 plus correction 1 uncommitted; change only what this brief names.

## The defect (material, introduced by correction 1)

`internal/landing/plain/depth.go` ~62-68 and ~76-80: `handIns` collects every queue line of each waiting goal, including hand-ins that main already contains (a landed line stays on the queue as "superseded" because landed is derived, not recorded). Once a waiting goal has an earlier hand-in on main, every other waiting goal built on later main "contains" it, the whole queue becomes one group, and the class is full whenever any member is full: the split by depth class (R-149-m1e) never happens. Admission already skips lines main contains (`batch.go` ~704-711); selection does not. Reproduction: goal a (tier 3) lands at a1, then hands in a2 on main; goal b (tier 1) cut from main, unrelated -> selected together as full (before correction 1: b alone, cheap).

## Fix by subtraction

When building the ancestry relation, skip any hand-in commit that main already contains, using the same containment check admission uses (`batch.go` ~704-711; share it, do not copy it). Everything else in the relation stays (current and superseded lines NOT on main count, both directions).

## Test

The read's reproduction (`zz_critic_landed_test.go` in `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/f77e7ebb-e1ab-4137-9935-206f37035290/scratchpad/mod`, a copy; port it into `depth_test.go` or `landing_depth_class_test.go` under the repository's naming): a landed at a1, a2 waiting, b unrelated cheap -> b alone as the cheap batch, then a at full (mutation: count landed hand-ins -> one full batch, test fails). Keep `TestLandingSelectAdapterSupersededAncestryKeepsFullBatch` green (a superseded hand-in NOT on main still ties).

## Checks

`go test -count=1 -timeout 30m ./internal/landing/plain`; cmd tests by name (`TestLandingSelect|TestSelectBatch|TestLandingDepth|TestFullDue|TestLandingProve|TestLandingPush`); every existing test using a seam you changed; `go run ./cmd/devgate static` (ignore the design-record ledger complaint; this worktree predates the goal's ledger commit). Never edit testing.json; never open metasystem.conf.local; do not commit. Report: diff --stat of this correction, each exit, the reproduction before/after.
