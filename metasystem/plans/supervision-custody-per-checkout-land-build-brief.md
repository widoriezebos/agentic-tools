Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal supervision-custody-per-checkout-land)
Date: 2026-09-05

# Build brief: the custody fix, corrected so no armed checkout is locked out

Goal `supervision-custody-per-checkout-land` (tier 3, approved under R-80-m2, Wido's word of 2026-09-05; a critic reviews before landing).

## What happened

Chain scc-build3 (preserved on branch `preserve/scc-build3-r1`, ten files) holds the custody work: the two-checkout invariant test, the selection by recorded checkout path, the drop-and-log reduction, the arm script, the suite self-check, the orchestration paragraph. Its critic (record `records/misc/supervision-custody-per-checkout-critique-cc3.md`, findings SCC-31 to SCC-36) rejected it on three material points, all accepted:

- SCC-31: owners now record the git top-level as their checkout path while every existing registry row holds the state root (the `metasystem` directory under the git top in template mode, which is how both seats and every worktree run). The guard in `internal/supervise/arming.go` compares recorded path with requested scope and refuses re-arm, generation replacement, shutdown and dead-owner takeover on mismatch before any liveness check. Landing it locks out every checkout armed by the previous binary.
- SCC-32: the guard refuses whenever the registry has no row for the lock owner's tag, and it runs before the liveness check; an owner that died before its first row is a permanent lockout.
- SCC-33: the suite self-check rejects only the suite shell's pid as a main identity; the stop-hook scenario resolves the seat's agent process through the ancestor walk and arms with it, which the new orchestration paragraph forbids and the check accepts.

## What to do

1. `git cherry-pick --no-commit preserve/scc-build3-r1`.
2. SCC-31: the recorded checkout path keeps the form the registry writers already use, the canonical state root; do not change what `RegistryLedger.CheckoutPath` and the owner command record. The guard compares the requested canonical state root with the recorded canonical state root, exactly, for the owner the reduction resolved; the tag stays a veto only. Extend the invariant test with an owner armed the way the previous binary armed it (a row holding the state root written before the change) and prove that re-arm, replacement, shutdown and takeover of that checkout still work and still never touch another.
3. SCC-32: the guard applies only to a live owner; when the lock names a dead owner, with or without a registry row, takeover proceeds as it did before this goal. Test it.
4. SCC-33: the self-check rejects any audited main pid that is not a process created inside the scenario's own bed (the bed-child shell or a process it started), so the seat's agent, the suite shell and any ancestor are refused; the stop-hook scenario creates its own held main. Test the self-check with a scenario that arms with the seat's pid and show it fails.
5. SCC-34 to SCC-36 as noted in the record: make the same-main and different-main sub-tests exercise a real difference or merge them; key the selection on rows that survive the registry contract's drops and compaction (not on rows compaction discards); remove the duplicated default registry path, give `Shutdown` the requested checkout it needs, and log reduction conflicts through the package's own logger rather than the standard logger to stderr.

## Verification

`gofmt -l`, `go vet`, `go run honnef.co/go/tools/cmd/staticcheck@2025.1` on `./internal/registry/... ./internal/supervise/... ./internal/up/... ./cmd/metasystem/`; `go test` on those four; `bash -n` on both scripts. Return within ninety minutes. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`. The orchestrator runs the supervision suite seat-side, re-arms this checkout with the new engine as the proof for SCC-31, and dispatches the critic.
