Working Mode: build
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Correction round on chain path-class-build1c (your reviewed tree
32636e35). The stamp review found one material regression,
PCM-CC4-001, in the root layout only (the installation is the
repository root, the git prefix is empty, the layout
metasystem/scripts/adopt.sh produces). Fold it; nothing else changes.

# The finding and the ruling

In the root layout the waiver rule in
metasystem/internal/validate/conformance.go (classifyWaiverPaths) asks
the ownership oracle before the manifest. The oracle's shipped
inventory (metasystem/internal/stateroot/owner.go, shippedInventoryPath)
does not name docs/project-rules.md, memory/README.md,
plans/README.md, records/README.md or records/goals/, so they answer
app-owned, ResolveRepositoryPath answers outside, and a prose waiver on
docs/project-rules.md is accepted where the deleted list refused it.

The orchestrator rules: the manifest decides first, ownership decides
only what the manifest does not name. In the root layout, for every
changed path: if the manifest has a row for the key, the row's class
decides exactly as in the vendored layout (behavior, ledger, runtime
and unclassified refused; record waivable); only a path with no row
falls to the ownership oracle, where app-owned means outside and
waivable, and installation-owned means unclassified and refused. The
certified design's section on the waiver consumer already says the
manifest is the one authority; this restores it for the root layout.

# The change

1. Reorder the root-layout branch of the waiver classification so the
   manifest lookup precedes the ownership answer, as ruled above.
2. Extend the root-layout test in
   metasystem/internal/validate/conformance_test.go with legs for
   docs/project-rules.md (refused), records/goals/ (refused),
   memory/README.md (refused), docs/application.md (no row, app-owned,
   waivable) and AGENTS.md (refused), all in the root layout.
3. No other product byte changes. Declare the boundary as the two files
   you touch, with the metasystem/ prefix.

# Gate

`go build ./...` clean; `go vet` and `gofmt -l` on internal/validate;
`go test ./internal/validate/ ./internal/pathclass/ -count=1` green where
the sandbox allows (the orchestrator replays the worktree-creating
tests outside the sandbox, KI-15); `bash scripts/agents/path-class-fixtures.sh`
green.

# Constraints

Wall-clock budget: 25 minutes. DESIGN-BEARING reach. R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the two files.

# Gap Rule

stop and report a gap; never fill it silently.
