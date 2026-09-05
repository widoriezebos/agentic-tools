Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal supervision-custody-per-checkout)
Date: 2026-09-05

# Build brief: the custody work with the second critic's corrections

Goal `supervision-custody-per-checkout` (tier 3, approved under R-79-m2; box 8 hours / 10 attempts / 1 active job / 3 review rounds). A code critic reviews before landing; this chain starts a fresh finding register, so the critic will number from SCC-31.

## What happened

Chain scc-build2 carried the custody work through two rounds and two critic rounds. Its second round is preserved on branch `preserve/scc-build2-r2` (eight files: the two-checkout invariant test and selection fix in `internal/registry` and `internal/supervise`, the arm script, the suite self-check, the orchestration doc line). The second critic (chain scc-build2-cc2) accepted two material findings that are not yet corrected, and its register cannot fold because its ids collided with the first critic's, so the corrections come through this fresh chain.

## What to do

1. `git cherry-pick --no-commit preserve/scc-build2-r2`; confirm the eight files.
2. Correct the first finding: in `internal/supervise/arming.go`, `requireOwnerCheckout` is called with the state root as both the requested and the recorded path, so its comparisons are tautologies and the only veto is a `strings.HasPrefix` of the owner tag against a slug of the git scope, which a sibling checkout with a hyphenated suffix (`/x/agentic-tools-m2` against `/x/agentic-tools`) or a worktree nested under the checkout satisfies. Read the checkout path recorded in the owner's own registry row, the row the reduction resolved for that owner, and compare it for exact equality with the requested canonical path, canonicalised the way the writer canonicalised it; the tag stays a veto only. Remove the dead pre-registry branch (both callers pass the same value twice). Extend the invariant test with an armed sibling checkout whose path extends the requested one by a suffix and an armed worktree nested under the checkout, and prove neither is ever selected.
3. Correct the second finding: `internal/registry/reduce.go` now refuses the whole registry when an owner tag or custody id appears under two checkout paths (new error returns around lines 166 to 177, 210, 290 and 327, and in `bindCustodies`), a new corruption class under REG-5 of `docs/design/supervision-registry.md`, which the design calls a contract change and which nothing names. Do not change the contract: a later record whose identity conflicts with an earlier owner's checkout path is dropped as sequence-illegal, as the reduction's own comment says, and the drop is logged with both paths; remove the new error returns.
4. Keep everything else from the preserved tree.

## Verification

`gofmt -l`, `go vet`, `go run honnef.co/go/tools/cmd/staticcheck@2025.1` on `./internal/registry/... ./internal/supervise/...`; `go test ./internal/registry/... ./internal/supervise/...`; `bash -n` on both scripts. Return within one hour. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`. The orchestrator runs the supervision suite seat-side.
