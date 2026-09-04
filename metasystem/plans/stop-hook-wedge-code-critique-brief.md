Working Mode: implement
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal stop-hook-wedge-on-enrollment-drift)
Date: 2026-09-04

# Review brief: the stop-hook wedge fix (chain shw-build1)

FINDING IDS: chain-unique, SHW-01, SHW-02, ... never F-n.

Round budget: 1 focused round, then at most one correction and its
re-review. R-60-m1's rule: material only if it changes what gets built
and names the artifact.

Threat model: the hook letting a turn end while the seat itself has
open work (the open-work refusal must keep blocking on every stop); a
cause that IS the seat's being treated as not the seat's; the
per-session refusal record leaking across sessions or being landed;
the deadline parent losing its overrun handling; a change to the
engine's stop-block JSON that other callers read. Out: taste; the
underlying steward and enrollment defects (their own items).

Scope: the computed diff of the implementer job under review.
Contract: metasystem/plans/stop-hook-wedge-build-brief.md; the goal
record metasystem/plans/goals/stop-hook-wedge-on-enrollment-drift.md.

# Mandate

1. A not-the-seat's refusal blocks once per cause per session, then
   surfaces as a systemMessage; the fixtures prove the second stop
   ends the turn and a new session starts fresh.
2. The deadline path follows the same rule and still kills the
   overrunning child as before.
3. The open-work refusal is unchanged and blocks on every stop.
4. The refusal record lives under artifacts/agents/supervision (runtime
   class) and is keyed by session and cause digest.
5. The fixture suite and the hook's syntax check are green.

If nothing material remains, say so; that closes the chain and the
fix lands.

# Constraints

Wall-clock budget: 30 minutes. Return per the code-critic schema with
the reviewedTree from validate conformance --stage review for job
shw-build1.

# Gap Rule

stop and report a gap; never fill it silently.
