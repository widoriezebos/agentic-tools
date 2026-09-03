Working Mode: build
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Follow-up on chain path-class-build2: your gap was correct. Section 7's
fixture for the unclassified detail, as written, contradicts section
3's floor precedence, because the manifest itself is behavior class and
the floor refuses the whole candidate before the unclassified check
runs. Continue the second part with the settlement below; everything in
metasystem/plans/path-class-manifest-build2-brief.md and its follow-ups
stands.

# The settlement

Precedence is set-wide and in this order: a behavior path anywhere in
the candidate refuses with direct-fix-floor-refused; then ledger,
runtime and unclassified refusals as section 3 states. The fixture
TestObserveUnclassifiedDetailFromBase takes your alternative shape: the
candidate changes only product.txt (a path the base manifest does not
classify), while the checked-out manifest outside the candidate is
altered to classify it; the evaluator must still answer
path-unclassified with the base manifest's detail, which proves that
classification reads the landing base and never the working tree.

# Then

Finish the remaining items of the build brief (the wrapper inputs in
land.sh and commit.sh, the end-to-end fixtures in land-fixtures.sh,
path-class-fixtures.sh and static-reproof-fixtures.sh), run the gate,
and declare every changed path with the metasystem/ prefix.

# Constraints

Wall-clock budget: 90 minutes. DESIGN-BEARING reach. R-31: no benchmarks.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the files you changed.

# Gap Rule

stop and report a gap; never fill it silently.
