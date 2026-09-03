Working Mode: build
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Promote the second landing bar: the landing evaluator refuses, for
agent commits, the code direct-fix-floor-refused, which under the
committed path-class manifest means exactly that a register-carriage or
exact-revert landing changed a behavior path. The first bar (R-40-m0)
and the manifest's own codes (R-64-m1) are enforced; this bar was
observed until the manifest's second part landed. The authorization is
the ruling row R-65-m1, the last row of metasystem/memory/rulings.md,
landed by the orchestrator before this round.

# The change

1. metasystem/scripts/agents/landing-promotion.json: add the string
   "direct-fix-floor-refused" to refuseCodes. Nothing else in the file.
2. The fixture that proves promotion in
   metasystem/scripts/agents/static-reproof-fixtures.sh or
   metasystem/scripts/agents/land-fixtures.sh (whichever already covers
   the promoted set) gains one leg: a register-carriage landing that
   changes a behavior path is refused with direct-fix-floor-refused,
   and the same landing with only record paths under a held goal
   passes.
3. Declare the boundary as the files you touch, with the metasystem/
   prefix.

# Gate

`go test ./internal/landing/ -count=1` green; the fixture script of
point 2 green; `gofmt -l` empty on internal/landing if touched.

# Constraints

Wall-clock budget: 20 minutes. DESIGN-BEARING reach (the change is one
line; the class comes from the chain law). R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the files you changed.

# Gap Rule

stop and report a gap; never fill it silently.
