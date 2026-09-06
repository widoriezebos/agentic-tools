Working Mode: implement
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-hook-budget-is-ours)
Date: 2026-09-06

# Review brief: the Stop hook budget, slice 2 (chain shbo-s2-build1-20260906)

FINDING IDS: chain-unique, SHS-01, SHS-02, ... never F-n.

Round budget: one focused round, then at most one correction and its
re-review. Material only if it changes what gets built and names the
artifact.

Threat model: an expiry recorded as a completion when the worker
actually completed first (the parent's kill lost the race and the
record was not ATTEMPTING); the new verb completing a record that
belongs to a later attempt; the health role reading a stale or
unmeasured number as a slow Stop, or missing an expiry because it read
the record field instead of the newest completed attempt; a threshold
that can be set at or above the ceiling; the role going dead on a
fresh checkout with no measurement; the unhealthy line not reaching
the alert channel; a change to the numbers or the measurement of slice
1; a fixture that cannot fail.

Out of scope: the hook's cost on an idle machine (goal
stop-hook-health-cost); the parent's liveness test (goal
stop-deadline-parent-trusts-ps); taste.

Scope: the computed diff of the implementer job under review.
Contract: metasystem/plans/stop-hook-budget-slice2-build-brief.md and
the goal record metasystem/plans/goals/stop-hook-budget-is-ours.md.

# Mandate

1. `steward hook-expire` completes only an ATTEMPTING supervision-hook
   record, with DEADLINE_EXPIRED and the elapsed on record and history,
   and refuses everything else unchanged; the parent calls it after the
   kill and before its log line; name the test for each.
2. The role reads the newest completed attempt, treats no measurement
   as alive with the stated reason, flags at or above the threshold and
   on expiry with the stated texts, and the threshold key is validated
   below sixty with default fifteen.
3. The unhealthy verdict reaches the alert channel through the existing
   path (say what you read to know).
4. Both fixture suites gain the assertions the brief named and each can
   fail on the old code.
5. Nothing from slice 1 changed; nothing outside the named owners.

If nothing material remains, say so; that closes the chain.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
shbo-s2-build1-20260906.

# Gap Rule

stop and report a gap; never fill it silently.
