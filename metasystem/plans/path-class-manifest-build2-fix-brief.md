Working Mode: build
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Follow-up on chain path-class-build2: both gaps were correct, both were
the brief's. Continue the second part of
metasystem/plans/path-class-manifest-design.md with the two points
settled below; everything else in
metasystem/plans/path-class-manifest-build2-brief.md stands.

# The two settlements

1. The promotion row is landed by the orchestrator: R-64-m1, the last
   row of metasystem/memory/rulings.md. Do not touch that file; your
   change to metasystem/scripts/agents/landing-promotion.json cites
   R-64-m1 where the promotion record names its authorization, the way
   the existing rows cite R-40-m0.
2. metasystem/internal/stateroot/owner.go joins the authorized boundary
   for exactly the two comment edits of the residual (state the
   constraint in the system's own terms; no goal name in code). Nothing
   else in that file changes.

# Boundary

The slice-2 boundary of the build brief plus
metasystem/internal/stateroot/owner.go, minus metasystem/memory/rulings.md.
Declare every changed path with the metasystem/ prefix.

# Gate

As the build brief states.

# Constraints

Wall-clock budget: 90 minutes. DESIGN-BEARING reach. R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the files you changed.

# Gap Rule

stop and report a gap; never fill it silently.
