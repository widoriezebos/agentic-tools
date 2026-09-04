Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal dispatch-engine-script-skew-carry)
Date: 2026-09-04

# Build brief: carry the dispatcher skew fix through a chain

Goal `dispatch-engine-script-skew-carry` (tier 1, approved by Wido's channel answer of 2026-09-04, R-75-m2; box 1 hour / 3 attempts / 1 active job / no review round), carrying the fix of goal dispatch-engine-script-skew-silent-exit. No critic in this chain; the orchestrator reviewed the diff.

## What happened

The first build chain produced the fix: the json verb exits 3 on an absent field, the dispatcher's `json_value` names the missing field and the remedy, `supervise status` reports the engine build stamp, and a dispatch preflight refuses when the engine commit is older than a checkout whose engine or agent scripts changed (six files, 133 insertions). Its return was rejected by the protocol only because the diffBoundary named paths without the repository prefix. The work is preserved on branch `preserve/dss-build2-r1`.

## What to do

In your worktree run `git cherry-pick --no-commit preserve/dss-build2-r1`, confirm the diff is those six files, run `gofmt -l` on the changed Go files and `go test ./cmd/metasystem/ -run 'JSON|Json'` and `bash -n` on both scripts, and return. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/` (for example `metasystem/scripts/agents/dispatch.sh`). Change nothing else. Do not run the fixture suite (KI-15; the orchestrator runs it seat-side).
