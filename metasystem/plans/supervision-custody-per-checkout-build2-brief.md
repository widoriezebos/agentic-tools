Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal supervision-custody-per-checkout)
Date: 2026-09-05

# Build brief: carry the custody work through a chain

Goal `supervision-custody-per-checkout` (tier 3, approved under R-79-m2; box 8 hours / 10 attempts / 1 active job / 3 review rounds). A code critic reviews before landing.

## What happened

The first chain (scc-build1) produced the work the build brief `plans/supervision-custody-per-checkout-build-brief.md` asked for, eight files: the two-checkout invariant test and selection fix in `internal/registry` (a new `selection.go`, `reduce.go`, `reduce_test.go`) and `internal/supervise` (`arming.go`, `arming_test.go`), the arm script, the supervision suite's self-check, and the orchestration doc line. It timed out at its two-hour cap in the moment it emitted its return, so the return was never recorded. The work is preserved on branch `preserve/scc-build1-r1`.

## What to do

In your worktree run `git cherry-pick --no-commit preserve/scc-build1-r1`, confirm the diff is those eight files, run `gofmt -l` and `go vet` on `./internal/registry/... ./internal/supervise/...`, `go run honnef.co/go/tools/cmd/staticcheck@2025.1` on them, `bash -n` on both scripts, and `go test ./internal/registry/...` plus the focused new tests in `internal/supervise` (`-run` the invariant test by name). Then return, within one hour: whatWasDone in three sentences naming the operation the test found crossing and the selection rule now enforced. Do not run the process-owning suite (KI-15; the orchestrator has). Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.
