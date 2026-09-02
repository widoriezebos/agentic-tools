Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal failed-job-attention)
Date: 2026-09-02

# Goal

Revision 2 of metasystem/plans/failed-job-attention-design.md: fold all
seven findings of
metasystem/records/misc/failed-job-attention-critique-r1.md (landed, in
your worktree), by id.

# Direction per finding

- FJA-R1-BIRTH-ABA: state the dedup key's honest dependency — until goal
  job-record-birth-token lands, the fallback digest's reuse exposure is
  declared with its bounded lifetime argument, and the design names the
  one-line upgrade when the token exists.
- FJA-R1-STOP-PREDICATE: stop claiming identity with the channel predicate
  — specify THIS design's stop predicate on the facts the tick already
  holds, and state the difference from the channel's explicitly.
- FJA-R1-CHANNEL-PARTIAL-FACTS: the episodes take THIS design's own
  identifier namespace and schema, never a partial record under the
  channel's final identifiers; the channel migrates by its own design.
- FJA-R1-PENDING-LIFECYCLE: one rule for notifications already queued when
  the off-switch flips (deliver-then-quiet or withdraw — decide, prove no
  orphan).
- FJA-R1-DIGEST-TRANSITION-LOSS: order the durable writes so a crash
  between episode write and digest append cannot lose the transition
  (derive-on-next-tick or write-ahead, choose and prove).
- FJA-R1-READ-BOUND: fix the NAG rule's read contract (a stat proves
  existence, not state — read what the rule needs or change the rule).
- FJA-R1-PROTOCOL-WRITER-PROOF: the fixture drives the real
  RecordProtocolError path, not a hand-written terminal record.

Consistency pass; self-grade; reject condition restated.

# Constraints

Wall-clock budget: 25 minutes. The seven folds only; the minimal-now
scope governor stands.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/failed-job-attention-design.md (that one file).

# Gap Rule

stop and report a gap; never fill it silently.
