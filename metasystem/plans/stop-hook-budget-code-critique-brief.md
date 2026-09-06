Working Mode: implement
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-hook-budget-is-ours)
Date: 2026-09-06

# Review brief: the Stop hook budget, slice 1 (chain shbo-build1-20260906)

FINDING IDS: chain-unique, SHB-01, SHB-02, ... never F-n.

Round budget: one focused round, then at most one correction and its
re-review. Material only if it changes what gets built and names the
artifact.

Threat model: the deadline parent no longer refuses on a genuine hang
(the fail-closed path broken, or the worker's own elapsed check
replacing the parent's); the worker deadline and the registered timeout
disagree in a way that lets the runtime kill the hook before the parent
answers (worker share not below the registered sixty, or the parent's
three seconds spent); an elapsed number that lies (measured from the
wrong start, negative, or written on one path and not another); a
hook-complete call that fails against an engine without the new flag or
a script without it; the component record made unreadable to older
readers (a required field, a validation that rejects existing records);
the fixture asserting a number that is always present even when the
measurement failed; the fixture's own wait ceilings tripping at the new
numbers; a change outside the brief.

Out of scope: the hook's cost on an idle machine (goal
stop-hook-health-cost); the live settings file at the repository root,
which this chain cannot carry and the orchestrator holds for Wido's
edit; the steward health role that reads the durations (slice 2).

Scope: the computed diff of the implementer job under review.
Contract: metasystem/plans/stop-hook-budget-build-brief.md and the goal
record metasystem/plans/goals/stop-hook-budget-is-ours.md.

# Mandate

1. The three Stop registrations under metasystem/scripts/enforcement say
   sixty; the parent's worker share is fifty-seven and derives from the
   budget variable; the comment names where the number lives.
2. On every emission path the hooks log line carries `elapsed=<n>s`
   measured from the parent's start, and the component record carries
   `lastStopElapsedSec`; the parent's expiry line carries elapsed too.
   Name the path that does not, if any.
3. `steward hook-complete` without `--elapsed-sec` still completes;
   a negative is refused; a record without the field still loads
   (name the test that proves each).
4. The fixture's deadline scenario overruns the worker and stays under
   the provider budget, and the two new assertions exist and can fail
   (say how each would fail on the old code).
5. Nothing outside the named owners changed.

If nothing material remains, say so; that closes the chain.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
shbo-build1-20260906-r2 (the correction round; the chain root is shbo-build1-20260906).

# Gap Rule

stop and report a gap; never fill it silently.
