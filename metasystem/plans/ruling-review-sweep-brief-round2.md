Working Mode: implement
Orchestrator Identity: m3 (lineage mac-m3, dispatch delegate under goal ruling-review-sweep)
Date: 2026-08-31

# Goal

One correction to your round-1 return, no code changes. The conformance
gate resolves boundary declarations from the git repository root, and the
project lives in the `metasystem/` subdirectory, so your `diffBoundary`
entries must carry that prefix. The round-1 brief named the paths without
it; that was the brief's defect, not yours.

# Workspace

The same job worktree. Touch NO source files. Only the returned
`diffBoundary` changes.

# Inputs

Your round-1 return. The changed files on disk are correct and stay
exactly as they are.

# Constraints

- No code, test, or documentation edits of any kind.
- Wall-clock budget: 5 minutes.

# Expected Return

The same version-2 implementer JSON as round 1, with `round` advanced and
`diffBoundary` reading exactly:

- metasystem/internal/steward/ruling_sweep.go
- metasystem/internal/steward/ruling_sweep_test.go

Evidence may carry forward round 1's entries unchanged plus one entry
showing `git diff --name-only` from the repository root listing exactly
those two paths.

# Acceptance Criteria

1. `diffBoundary` lists exactly the two prefixed paths above.
2. `git diff --name-only` from the repository root shows exactly those
   two paths.
3. No file content changed since round 1.

# Gap Rule

stop and report a gap; never fill it silently.
