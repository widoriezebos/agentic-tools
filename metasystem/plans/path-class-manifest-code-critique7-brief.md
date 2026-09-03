Working Mode: implement
Orchestrator Identity: m1+main-1788333680-2840-7f79f4 (dispatch delegate under goal path-class-manifest)
Date: 2026-09-03

# Goal

Closing code review of the path-class manifest, first part, chain
path-class-build1c, at the review budget of the chain (three closing
reviews so far: PCM-CC4, PCM-CC5, PCM-CC6). The round evidence holds
diff.patch and review.json with the reviewed tree. The orchestrator
dispositioned PCM-CC6-001 under R-60-m1 (at the budget, the agreed
parts build and the still-open point becomes a named obligation): the
inventory's shortfall against adoption's full install set predates
this feature, regresses nothing the base protected, affects only the
root layout no fleet machine runs, and is now goal
adoption-inventory-from-install-set in the backlog; the design's
resolution paragraph in metasystem/plans/path-class-manifest-design.md
is narrowed to the exact list the inventory carries; the over-claiming
comments in metasystem/internal/stateroot/owner.go are corrected
(brief metasystem/plans/path-class-manifest-build1c-fix4-brief.md).

# Mandate

1. The tree equals your reviewed tree 4e766443 except the two declared
   files, whose changes are prose only (comments and one test name).
2. The design paragraph now states exactly what the inventory carries
   and names the follow-up goal; the owner.go comments no longer claim
   lockstep with adoption or the tracer's source of truth.
3. PCM-CC6-003 (directory queries answer application-owned in the root
   layout) is a recorded residual for the follow-up goal; it affects no
   gate.
4. The orchestrator's gate replay on the tree is at
   metasystem/artifacts/agents/path-class-build1c/orchestrator-gate-replay.txt.

Judge the tree as built for the vendored layout the fleet runs and for
the root layout as the narrowed design now states it. A finding is
material only if it changes what gets built and names the artifact; a
finding that re-raises the dispositioned shortfall is not material
unless it shows a regression against the base. If nothing material
remains, say so; that closes the chain.

# Constraints

Wall-clock budget: 20 minutes. Return per the code-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
