Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal return-normalisation-hides-boundary-mismatch)
Date: 2026-09-04

# Build brief: path normalisation must not hide an undeclared changed path

Goal `return-normalisation-hides-boundary-mismatch` (tier 2, approved under R-78-m2, Wido's word of 2026-09-05; box 4 hours / 6 attempts / 1 active job / 2 review rounds). No critic in this chain.

## The defect

`scripts/agents/dispatch-fixtures.sh` line 2558, leg `diff-boundary-mismatch`: the leg writes an untracked `source.txt` at the conformance job's workspace root, runs `validate conformance --root <agent repo> --stage review --job conformance`, and expects the refusal "changed paths fall outside the cumulative implementation boundary" (`internal/validate/conformance.go` line 474); it then sets the return's `diffBoundary` to `["source.txt"]` and expects the review to pass. Since the path-form landing (b7d119b3), `checkDiffBoundary` in `internal/validate/returncomplete.go` normalises metasystem-relative entries that resolve under `metasystem/` and refuses the rest with DIFF_BOUNDARY_INVALID, and the leg now fails with "did not report" that text. Seen seat-side on m2 2026-09-04 18:47Z.

## What to build

Reproduce the leg in a unit test (a workspace with a file at its root, a return whose boundary omits it, then one that declares it as `source.txt`) and read the fixture's agent repository layout to learn what "metasystem/" means there. Then fix the validator so that: an undeclared changed path is reported with the exact text above, whatever the boundary's path form; the normalisation rewrites only entries that resolve to an existing file under the metasystem root and leaves the rest to the existing regex rule; and a declared entry that the fixture's layout treats as valid (workspace-root `source.txt` in that repository) passes as it did before b7d119b3. If the second and third goals conflict, the fixture's pre-b7d119b3 behaviour wins and the normalisation narrows.

## Verification

`go test ./internal/validate/...` with the new test; `gofmt -l`, `go vet`, `go run honnef.co/go/tools/cmd/staticcheck@2025.1 ./internal/validate/...`. The orchestrator runs `dispatch-fixtures.sh` seat-side. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.

## Bounds

Touch `internal/validate` and its tests only; the fixture leg stays as it is.
