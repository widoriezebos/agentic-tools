Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-13

# Review brief: the last read of the carried chain, over round 11

FINDING IDS: chain-unique across this goal's critic chains; GAWR-C1-01 to
GAWR-C1-12 are taken. GAWR-C1-11 and GAWR-C1-12 are returned under their
OWN identifiers with `material: false` if resolved, `material: true` if
they still stand; a new defect gets GAWR-C1-13 onward.

Why this review exists: chain gawr-build1 (slices 1 and 2 of this goal)
was certified on 2026-09-10, carried onto today's trunk over rounds 6 to
8, built design revision 5 in round 10, and built revision 6 with the
missing fixture assertions in round 11. Three reads have run; this is the
read of the terminal round, the one that lets the chain close and land.

Round 11 reports:
- GAWR-C1-11: both counselor ownership readers in
  metasystem/internal/landing/observe.go now also read the abandoned map,
  with live and done matching unchanged, and
  TestAbandonedCounselorAppendsKeepOwnership in
  metasystem/internal/landing/hcl_carried_test.go covers each register
  alone and together, ownership in live, done and abandoned records, a
  later carry owned by another goal, missing or changed evidence, a
  non-human-carried waiver and a forbidden rewrite.
- GAWR-C1-12: the two lifecycle fixtures in
  metasystem/internal/goal/carry_lifecycle_test.go now prove the frozen
  stop-fence lines byte-identical through abandonment and waiver, a carry
  refused with carry-debt-unpaid while the debt is open, the next carry
  permitted after discharge, a wrong-chain discharge refused, and the
  ordinary obligation left open by a human-carried waiver.

Specification: metasystem/plans/goal-abandoned-with-a-reason-design.md
(revision 6). The whole landing package passed in the round; the seat runs
the whole goal package and the goal-cli bed outside the delegate sandbox
and will supply those results.

Round budget: 1 focused round. A finding is material only if the code or
tests must change before this lands, and it names the artifact.

# Mandate

1. Return GAWR-C1-11 and GAWR-C1-12 under their identifiers: is each
   resolved as revision 6 and section 13 state it?
2. The ownership readers: can the abandoned map now let a line be owned by
   a goal that should not own it, transfer ownership to a successor or to
   the landing's own goal, or grant any landing authority to an abandoned
   goal? Is every check that ran before still enforced on each path?
3. Anything rounds 6 to 10 settled that round 11 disturbed.
4. Nothing else.

# Expected Return

The code-critic return schema, findings sorted by materiality, each with
file:line evidence and its rigor row. Every path in your return starts
with `metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
