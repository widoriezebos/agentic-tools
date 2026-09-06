Working Mode: implement
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-hook-health-cost)
Date: 2026-09-06

# Review brief: the health preview stops re-reading the world (chain shhc-build1-20260906)

FINDING IDS: chain-unique, SHC-01, SHC-02, ... never F-n.

Round budget: one focused round, then at most one correction and its
re-review. Material only if it changes what gets built and names the
artifact.

Threat model, in order of harm: the spend fence is Wido's token and
money ceiling, so a cache that hides spend is the worst outcome (a
terminal job record cached under a stale key, a transcript cursor that
skips bytes, a delegate-set change that leaves excluded lines counted
or included lines dropped, a partial trailing line counted twice or
never); the incremental ledger differing in any byte from the full
parse for the same files; the exact-slug and first-cwd rule excluding a
transcript the old scan counted (a session started in a subdirectory of
the checkout, a first line without cwd, a file whose first assistant
line comes late); two health calls at once (steward tick and Stop hook)
corrupting a cache file; a cache write failure that breaks measurement
instead of degrading to a full parse; unbounded growth of the cache
directory; the health line text changed; a duration field that lands on
the line instead of in the JSON; a change outside the named owners.

Out of scope: the Stop hook script and its deadline (goal
stop-hook-budget-is-ours); the transcript slug matching's other seats'
tokens beyond what this brief changes; taste.

Scope: the computed diff of the implementer job under review.
Contract: metasystem/plans/stop-hook-health-cost-build-brief.md and the
goal record metasystem/plans/goals/stop-hook-health-cost.md.

# Mandate

1. For the same set of files, the cached and incremental measurement
   yields a byte-identical ledger to the full parse; name the test that
   proves each of the four brief cases and any case the tests miss.
2. No cache can hide spend: every key that decides "reuse" is named and
   sufficient, and every failure path degrades to a full parse rather
   than to a smaller number.
3. Transcript selection: exact slug boundary and first-cwd membership
   never drop a file the old scan counted for this checkout; say how
   you know.
4. The warm preview reads no transcript bytes and no terminal job
   record (the counters the return names), and the measurement test's
   bound is real, not a sleep-free tautology.
5. Nothing outside the named owners changed; the health line is
   byte-identical for the same roles.

If nothing material remains, say so; that closes the chain.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
shhc-build1-20260906.

# Gap Rule

stop and report a gap; never fill it silently.
