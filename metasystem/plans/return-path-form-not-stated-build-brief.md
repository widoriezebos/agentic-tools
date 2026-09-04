Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal return-path-form-not-stated)
Date: 2026-09-04

# Build brief: the return's path form is stated in the prompt and tolerated by the validator

Goal `return-path-form-not-stated` (tier 1, approved under R-76-m2, Wido's word of 2026-09-04; box 1 hour / 3 attempts / 1 active job / no review round). No critic in this chain.

## The defect

An implementer's return is rejected with `DIFF_BOUNDARY_INVALID: diffBoundary entry "<path>" must match ^metasystem/.+` when a path is named relative to the metasystem directory, which is the implementer's working directory and the form every brief uses. Nothing in the delegate prompt says return paths are repository-root relative. Two chains were lost this way on 2026-09-04 (each cost a preserve branch and a carry chain) before every brief started carrying the sentence by hand.

## What to build

1. In the prompt the dispatcher assembles (`scripts/agents/dispatch.sh` and the templates `scripts/agents/templates/brief.md` and `scripts/agents/templates/follow-up.md`), one sentence next to the return schema: "Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`."
2. In the return validator that emits `DIFF_BOUNDARY_INVALID` (`internal/validate/returncomplete.go`, around line 247), when every offending entry resolves to an existing file under `metasystem/` in the worktree, normalise the entries by prefixing `metasystem/` (record the normalisation in the round's protocol note) instead of failing the round; an entry that resolves nowhere still fails with the same code.

## Verification

A test in the validator's package for: metasystem-relative entries that exist are normalised; an unknown entry still fails; already-correct entries are untouched. Run `bash -n` on the script, `gofmt -l`, `go vet` and `go run honnef.co/go/tools/cmd/staticcheck@2025.1` on the touched package.

## Bounds

Touch the dispatcher's prompt assembly, the template, the validator and its test only. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.
