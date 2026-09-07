Working Mode: implement
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-hook-open-work-refusal-repeats)
Date: 2026-09-07

# Review brief, round two: the fold on chain show-build1d-20260907

FINDING IDS: chain-unique, continue the series (SHO-08 onward); never
F-n; a re-opened round-one finding keeps its id.

Round one (metasystem/plans/stop-hook-open-work-refusal-repeats-code-critique-brief.md,
critic show-cc1-20260907) returned five material findings; the fold
brief metasystem/plans/stop-hook-open-work-refusal-repeats-fold-brief.md
carried them to the implementer, whose round-two return is at
metasystem/artifacts/agents/show-build1d-20260907/rounds/2/return.json.
The contract is unchanged (the build brief and the goal record
metasystem/plans/goals/stop-hook-open-work-refusal-repeats.md).

Scope: the computed diff of round two, the whole change against main.
Threat model as in round one. This is the re-review after the one
correction the tier allows; material only if it changes what gets
built and names the artifact.

# Mandate

1. SHO-01 closed: the VERDICT (report turn-verdict) treats an open
   chain as work in flight; a test on the verdict proves it.
2. SHO-02 closed: every read failure of the seen-state fails open with
   one visible line; no exit 1; tests per failure shape.
3. SHO-03 closed: the deadline path reads and never writes; after an
   expiry the line still blocks once (fixture leg corrected).
4. SHO-04 closed: the key is the digest of the full plan line, not the
   clipped display.
5. SHO-05 closed: stale entries are dropped on write; tests for an
   edited plan and a removed plan.
6. Nothing widened; the file set is the round-one set.

If nothing material remains, say so; that closes the chain and the fix
lands.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
show-build1d-20260907-r2 (its review.json sits in rounds/2).

# Gap Rule

stop and report a gap; never fill it silently.
