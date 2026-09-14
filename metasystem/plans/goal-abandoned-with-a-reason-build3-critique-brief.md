Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-14

# Review brief: the landing side of the goal, carried and folded

FINDING IDS: GAWR-C3-01 onward.

Why this review exists: chain gawr-build3 is slice 3 of goal
goal-abandoned-with-a-reason, the landing side of section 4a of
metasystem/plans/goal-abandoned-with-a-reason-design.md (now revision 7).
It was certified on 2026-09-10 and never landed. Slices 1 and 2 landed at
345d809c after their own carry, so trunk already has abandon, the engine
floor, reopen from abandoned and the abandoned carry. Round 3 carried this
slice onto trunk and resolved six conflicted files; round 4 fixed what the
seat's out-of-sandbox runs found and built revision 7. This read is over
round 4, the terminal round, and it lets the chain close and land.

What rounds 3 and 4 report:
- The carry kept trunk's verb registration beside the new held verb, its
  observation tests, every refusal row, its proof, lease and carry
  behaviour in commit.sh, and all 26 bed scenarios, adding the
  abandonment route as the 27th and the held check before each push.
- tier-one had regressed: the reduced fixture wrapper parsed but did not
  forward its root-job argument, and the leg lacked a readable tier-one
  root and policy. Round 4 forwards the argument, seeds the root, and
  restores trunk's strict rule that root job, goal and test receipt must
  all be present.
- Revision 7: the promotion reader accepts versions 1 and 2, keeps
  version 1's vocabulary, adds exactly goal-binding-missing,
  goal-binding-mismatch and goal-revision-moved for version 2, and
  refuses a higher version; the policy record becomes version 2 with every
  existing entry kept; receipt-cutover proves the old and new readers
  disagreeing, with the pinned engine and the page's repair text.
- The goal-state fixture switch is centralised in one helper, which now
  calls `goal abandon` because slices 1 and 2 are on trunk.

Evidence in hand: the delegate ran all 27 land bed scenarios green, the
whole landing package, and the fast gate. The seat ran the landing package
and the command selection outside the delegate sandbox on the carried
tree, both green, including two rearm tests the sandbox cannot run.

Round budget: 1 focused round. A finding is material only if the code or
tests must change before this lands, and it names the artifact.

# Mandate

1. The six conflict joins: does each keep both trunk's behaviour and the
   certified behaviour, with nothing lost from either side and nothing
   added beyond them? Name any place where trunk's rule was relaxed to
   make a fixture pass, tier-one especially.
2. The versioned promotion record: can a version-1 reader ever accept a
   version-2 record or the reverse; can a valid version-1 record lose its
   original meaning under a version-2 reader; does any path accept a
   version above its maximum; is the refusal whole-record as the page
   says?
3. The held check before each push, and the goal-and-revision binding: can
   a landing pass them by accident, or be refused for a goal that is
   lawfully claimed and current?
4. Nothing else. Slices 1 and 2 are landed and out of scope.

# Expected Return

The code-critic return schema, findings sorted by materiality, each with
file:line evidence and its rigor row. Every path in your return starts
with `metasystem/`.

# Gap Rule

stop and report a gap; never fill it silently.
