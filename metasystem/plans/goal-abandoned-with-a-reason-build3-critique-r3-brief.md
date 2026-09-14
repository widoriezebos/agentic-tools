Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-14

# Review brief: the last read of the landing side, over round 6

FINDING IDS: chain-unique; GAWR-C3-01 to GAWR-C3-10 are taken. C3-08 and
C3-10 are returned under their OWN identifiers with `material: false` if
resolved, `material: true` if they still stand; a new defect gets
GAWR-C3-11 onward.

Why this review exists: your last read resolved C3-01, C3-02 and C3-03 and
found C3-08, the range walk that runs back through history when the
fetched base is not an ancestor of the tip. Round 6 answers it and the
message nuance C3-10; it is the terminal round, and this read lets the
chain close and land.

What round 6 reports: the re-check now asks Git once for the oldest-first
first-parent ancestry path, validates every returned parent edge, refuses
a moved or unrelated base with the page's own sentence naming the base and
the tip, and still identifies a real merge inside the proposed range. The
regression failed on the old implementation with 99 Git calls and passes
with three: two commit resolutions and one range query. On the real deep
repository the moved-base refusal now takes 0.41 seconds, where the reader
measured 20.9 seconds. For C3-10, every commit begins with an ok candidate
outcome, so a later held commit replaces an earlier goal-free outcome while
a final goal-free commit still reports goal-free.

Evidence in hand: the whole landing package, both public beds (27 landing
scenarios and the static re-proof bed) and the fast gate passed in the
round; the seat is running the packages and both beds outside the delegate
sandbox on the same tree.

Specification: metasystem/plans/goal-abandoned-with-a-reason-design.md,
revision 7.

Round budget: 1 focused round. A finding is material only if the code or
tests must change before this lands, and it names the artifact.

# Mandate

1. Return C3-08 and C3-10 under their identifiers: is each resolved as
   stated, and does its test prove the behaviour?
2. The new range query: can it miss a merge inside the range, accept a
   base that is not a first-parent ancestor, or mis-order the commits it
   judges? Is every parent edge it trusts actually validated, and what
   does it do when Git answers nothing or answers a truncated path?
3. Does the outcome change of C3-10 alter any exit code or refusal, or
   only the words?
4. Anything earlier rounds settled that round 6 disturbed. Nothing else.

# Expected Return

The code-critic return schema, findings sorted by materiality, each with
file:line evidence and its rigor row. Every path in your return starts
with `metasystem/`; a file this chain adds is named without that prefix.

# Gap Rule

stop and report a gap; never fill it silently.
