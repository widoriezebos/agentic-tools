Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal supervision-custody-per-checkout)
Date: 2026-09-05

# Follow-up: the second critic's register, two material findings accepted

The second critic's register is in chain scc-build2-cc2 (findings SCC-01 to SCC-05; read its return for the full text). Accepted:

- SCC-01 (high): in `requireOwnerCheckout` both callers pass the state root as both the requested and the recorded path, so the two comparisons are tautologies, and the only veto left is `strings.HasPrefix` on the owner tag against a slug of the git scope, which a sibling checkout with a hyphenated suffix (`/x/agentic-tools-m2` against a request for `/x/agentic-tools`) or a worktree nested under the checkout satisfies. Fix: read the checkout path recorded in the owner's own registry row (the row the reduction resolved for that owner) and compare it, canonicalised the way the writer canonicalised it, for exact equality with the requested canonical path; the tag remains a veto only. Remove the dead pre-registry branch the third finding names. Extend the invariant test with a sibling checkout whose path extends the requested one by a suffix and with a worktree nested under the checkout, both armed, and prove neither is selected.
- SCC-02 (medium): the reduction now refuses the whole registry when an owner tag or custody id appears under two checkout paths, a new corruption class under REG-5 of `docs/design/supervision-registry.md`, which the design names a contract change, and nothing names it. Do not change the contract: a later record whose identity conflicts with an earlier owner's checkout path is dropped as sequence-illegal, exactly as the reduction's own comment says, and the drop is logged with both paths; remove the new error returns in `reduce.go` and `bindCustodies`.

SCC-03, SCC-04 and SCC-05 are noted, no separate change. Keep everything else. Run `gofmt -l`, `go vet`, staticcheck on `./internal/registry/... ./internal/supervise/...`, `go test ./internal/registry/... ./internal/supervise/...`, `bash -n` on both scripts. Return within one hour. Every path in your return is relative to the repository root (starting with `metasystem/`). The orchestrator runs the supervision suite seat-side.
