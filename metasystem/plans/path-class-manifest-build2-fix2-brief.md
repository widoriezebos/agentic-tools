Working Mode: build
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Follow-up on chain path-class-build2: your gap was correct and it was
the brief's. metasystem/scripts/agents/landing-promotion.json has no
authorization field and gets none. Continue the second part with this
settlement; everything else in
metasystem/plans/path-class-manifest-build2-brief.md and
metasystem/plans/path-class-manifest-build2-fix-brief.md stands.

# The settlement

The promotion file keeps its shape exactly: schemaVersion 1 and
refuseCodes as an array of code strings. Append the nine codes of
design section 6 to refuseCodes; no new field, no objects, no schema
version change, and promotion.go does not validate anything against
the rulings register. The authorization is the ruling row R-64-m1 in
metasystem/memory/rulings.md; the orchestrator cites it in the landing
message, the way R-40-m0 was cited when the first two codes were
promoted. Code carries no provenance.

# Boundary

As the previous follow-up states. Declare every changed path with the
metasystem/ prefix.

# Gate

As the build brief states.

# Constraints

Wall-clock budget: 90 minutes. DESIGN-BEARING reach. R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the files you changed.

# Gap Rule

stop and report a gap; never fill it silently.
