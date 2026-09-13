Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-13

# Review brief: the fold of the carry read, round 7

FINDING IDS: chain-unique; GAWR-C1-01 to GAWR-C1-09 are taken. A finding
of round 6 that this fold resolved is returned under its OWN identifier
with `material: false`; one that still stands keeps its identifier with
`material: true`; a new defect gets GAWR-C1-10 onward. Never report a
resolution as prose inside another finding.

Why this review exists: the read of the carry (gawr-carry1-read1) found
five material defects. Round 7 folded four; GAWR-C1-05 is reserved for a
design decision and is NOT under review here. What round 7 reports:

- GAWR-C1-03: abandon now clears the landing binding together with the
  claim and the obligation, with a test that opens, approves, claims,
  marks land-ready and abandons.
- GAWR-C1-04: dependency rewriting now moves a blocker park to the
  carried successor or to another unfinished blocker, and records an
  automatic unpark when a waiver removes the last unfinished blocker;
  the carried, waived and also cases are tested.
- GAWR-C1-01: the goal-cli scenario opens its parent and successor as
  human-origin goals and splits the human-origin parent under the
  fixture's terminal authority, to satisfy trunk's seat-open and
  split-ratification rules.
- GAWR-C1-02: the product keeps `abandoned=N` in the list summary and
  trunk's archive-and-prune check now expects it.
- GAWR-C1-06 (not material): the test-local `git init` stays, because
  removing it fails enrollment in that worktree.

Round budget: 1 focused round. A finding is material only if the code or
tests must change before this lands, and it names the artifact.

# Mandate

1. Return GAWR-C1-01, GAWR-C1-02, GAWR-C1-03 and GAWR-C1-04 under their
   identifiers: is each resolved, and does its test prove the behaviour
   rather than the implementation's own shape?
2. The park repair of GAWR-C1-04: can a dependent be left parked with no
   unfinished blocker, unparked while a blocker remains, or moved to a
   blocker that is itself abandoned or done? Does the automatic unpark
   follow trunk's own returnBlockerParks rule?
3. Does clearing the landing binding in abandon lose anything a resume or
   a landing slot owner needs, and does it leave the record valid for
   every state a goal can be abandoned from?
4. Nothing else. GAWR-C1-05 is out of scope.

# Expected Return

The code-critic return schema, findings sorted by materiality, each with
file:line evidence and its rigor row. Every path in your return starts
with `metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
